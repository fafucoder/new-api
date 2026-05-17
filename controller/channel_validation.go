// Package controller — channel_validation endpoint
//
// Runs the multi-step Claude authenticity probe defined in
// service/channel_validation. Open to any authenticated user; admins pick
// a specific channel by id, regular users pick a model and the backend
// resolves it to an Anthropic channel they're authorised to use (via the
// abilities table + their user.Group).
//
// Each run is persisted as a ChannelValidationRecord. Users can read
// back their own history; admins see everyone's. The list endpoint
// omits the bulky result_json blob — clients fetch a single record by
// id to inspect the full ValidationResult.
package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/channel_validation"

	"github.com/gin-gonic/gin"
)

// channelValidationRequest is the JSON body posted by the frontend. Admin
// callers pass channel_id; regular users pass only model (channel_id is
// ignored when the caller is not an admin).
type channelValidationRequest struct {
	ChannelID      int    `json:"channel_id"`
	Model          string `json:"model"`
	MaxTokens      int    `json:"max_tokens"`
	RunMultiTurn   bool   `json:"run_multi_turn"`
	RunTamper      bool   `json:"run_tamper"`
	ForcePadding   bool   `json:"force_padding"`
	Step2Prompt    string `json:"step2_prompt"`
	CrossChannelID int    `json:"cross_channel_id"`
}

// PostChannelValidationRun is the HTTP handler that triggers a validation
// suite. The suite runs synchronously; clients should allow ~3-5 min for
// the worst-case Step1+Step2+Step3 round-trips.
func PostChannelValidationRun(c *gin.Context) {
	var req channelValidationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model is required"})
		return
	}

	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser

	channel, err := resolveValidationChannel(c, &req, isAdmin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if channel == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if channel.Status == common.ChannelStatusManuallyDisabled {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "channel is manually disabled"})
		return
	}

	// Non-admins are not allowed to specify a cross-channel id, since they
	// can't pick channels at all. Strip it defensively.
	crossID := req.CrossChannelID
	if !isAdmin {
		crossID = 0
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	opts := channel_validation.Options{
		Model:          req.Model,
		MaxTokens:      req.MaxTokens,
		RunMultiTurn:   true,
		RunTamper:      true,
		ForcePadding:   req.ForcePadding,
		Step2Prompt:    req.Step2Prompt,
		CrossChannelID: crossID,
	}
	result, err := channel_validation.Run(ctx, channel, opts)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Persist the run as a record before masking, so admin lookups still
	// see the full channel context.
	if record, perr := persistValidationRecord(c, channel, result); perr != nil {
		common.SysError("failed to persist channel_validation record: " + perr.Error())
	} else if record != nil {
		result.RecordID = record.Id
	}

	// Non-admin callers must never see channel ids or names — masking
	// happens here, not inside the suite (so the admin response stays
	// rich).
	if !isAdmin {
		result.ChannelID = 0
		result.ChannelName = ""
		result.BaseURL = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
		"view":    viewLabel(isAdmin),
	})
}

// resolveValidationChannel returns the channel that the validation suite
// should target. Admins specify it by id; regular users specify a model
// and the backend picks the highest-priority Anthropic channel serving
// that model within their user group.
func resolveValidationChannel(c *gin.Context, req *channelValidationRequest, isAdmin bool) (*model.Channel, error) {
	if isAdmin {
		if req.ChannelID <= 0 {
			return nil, errors.New("channel_id is required for administrators")
		}
		channel, err := model.GetChannelById(req.ChannelID, true)
		if err != nil || channel == nil {
			return nil, errors.New("channel not found")
		}
		if channel.Type != constant.ChannelTypeAnthropic {
			return nil, errors.New("model validation currently supports Anthropic-direct channels only (channel_type=14)")
		}
		return channel, nil
	}

	userID := c.GetInt("id")
	groupStr, err := model.GetUserGroup(userID, true)
	if err != nil || groupStr == "" {
		return nil, errors.New("your account has no usable group; ask an administrator")
	}
	groups := model.SplitUserGroups(groupStr)
	if len(groups) == 0 {
		return nil, errors.New("your account has no usable group; ask an administrator")
	}

	channel, err := findEnabledAnthropicChannelForGroups(req.Model, groups)
	if err != nil {
		return nil, err
	}
	return channel, nil
}

// findEnabledAnthropicChannelForGroups returns the highest-priority
// enabled Anthropic channel (channel_type=14) serving modelName for any
// of the given user groups. Returns an error when no qualifying channel
// exists, so the caller can surface a clear message.
func findEnabledAnthropicChannelForGroups(modelName string, groups []string) (*model.Channel, error) {
	channelID, err := model.FindEnabledChannelForModelByType(modelName, groups, constant.ChannelTypeAnthropic)
	if err != nil {
		return nil, err
	}
	if channelID <= 0 {
		return nil, errors.New("no enabled Anthropic channel for that model in your group")
	}
	return model.GetChannelById(channelID, true)
}

func viewLabel(isAdmin bool) string {
	if isAdmin {
		return "admin"
	}
	return "user"
}

// GetChannelValidationModels lists the Claude models the current caller
// is authorised to validate. Admins see every Claude model served by any
// enabled Anthropic channel; users see only models their group can use.
func GetChannelValidationModels(c *gin.Context) {
	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser

	var groups []string
	if !isAdmin {
		userID := c.GetInt("id")
		groupStr, err := model.GetUserGroup(userID, true)
		if err == nil {
			groups = model.SplitUserGroups(groupStr)
		}
	}

	models, err := model.ListClaudeModelsForGroups(groups, isAdmin)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"view":   viewLabel(isAdmin),
			"models": models,
		},
	})
}

// GetChannelValidationChannelModels returns the model list configured on
// a single Anthropic channel. Admin only — regular users don't pick
// channels and don't need this. The frontend uses this to cascade the
// "channel → model" picker in the admin form.
func GetChannelValidationChannelModels(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid channel id"})
		return
	}
	channel, err := model.GetChannelById(id, true)
	if err != nil || channel == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "channel not found"})
		return
	}
	if channel.Type != constant.ChannelTypeAnthropic {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "channel is not Anthropic type"})
		return
	}
	models := channel.GetModels()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"channel_id":   channel.Id,
			"channel_name": channel.Name,
			"models":       models,
		},
	})
}

// GetChannelValidationRecords returns paginated detection history.
// Users see only their own runs; admins see everyone's. The list omits
// result_json — call the single-record endpoint to read it.
func GetChannelValidationRecords(c *gin.Context) {
	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	verdict := strings.TrimSpace(c.Query("verdict"))
	modelName := strings.TrimSpace(c.Query("model"))

	filter := model.ChannelValidationRecordFilter{
		Verdict:  verdict,
		Model:    modelName,
		Page:     page,
		PageSize: pageSize,
	}
	if !isAdmin {
		filter.UserId = c.GetInt("id")
	}

	records, total, err := model.ListChannelValidationRecords(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// For non-admin callers, scrub channel identifiers from list rows.
	if !isAdmin {
		for i := range records {
			records[i].ChannelId = 0
			records[i].ChannelName = ""
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":     records,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"view":      viewLabel(isAdmin),
		},
	})
}

// GetChannelValidationRecordDetail returns a single record including
// result_json. Users can only read their own; admins read any.
func GetChannelValidationRecordDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid record id"})
		return
	}

	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser
	scope := 0
	if !isAdmin {
		scope = c.GetInt("id")
	}

	record, err := model.GetChannelValidationRecord(id, scope)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "record not found"})
		return
	}

	// Decode result_json into the full ValidationResult so the frontend
	// can render it the same way as a fresh run.
	var result channel_validation.ValidationResult
	if record.ResultJson != "" {
		if err := common.UnmarshalJsonStr(record.ResultJson, &result); err != nil {
			common.SysError("failed to decode channel_validation record " + idStr + ": " + err.Error())
		}
	}
	if !isAdmin {
		record.ChannelId = 0
		record.ChannelName = ""
		result.ChannelID = 0
		result.ChannelName = ""
		result.BaseURL = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"record": record,
			"result": &result,
			"view":   viewLabel(isAdmin),
		},
	})
}

// DeleteChannelValidationRecordHandler removes a record. Users can
// delete their own; admins can delete any.
func DeleteChannelValidationRecordHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid record id"})
		return
	}

	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser
	scope := 0
	if !isAdmin {
		scope = c.GetInt("id")
	}

	affected, err := model.DeleteChannelValidationRecord(id, scope)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if affected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "record not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// persistValidationRecord stores a completed validation run. The full
// ValidationResult is serialized into result_json after trimming the
// largest debug fields so the row fits comfortably in a MySQL TEXT
// column (64KB). Returns the persisted record so the caller can echo
// its id back to the frontend.
func persistValidationRecord(c *gin.Context, channel *model.Channel, result *channel_validation.ValidationResult) (*model.ChannelValidationRecord, error) {
	if channel == nil || result == nil {
		return nil, nil
	}
	abridged := abridgeResultForStorage(result)
	payload, err := common.Marshal(abridged)
	if err != nil {
		return nil, err
	}
	record := &model.ChannelValidationRecord{
		UserId:      c.GetInt("id"),
		Username:    c.GetString("username"),
		ChannelId:   channel.Id,
		ChannelName: channel.Name,
		Model:       result.RequestedModel,
		Verdict:     result.Verdict,
		OK:          result.OK,
		DurationMs:  result.DurationMs,
		Summary:     result.Summary,
		ResultJson:  string(payload),
		CreatedTime: time.Now().Unix(),
	}
	if err := model.CreateChannelValidationRecord(record); err != nil {
		return nil, err
	}
	return record, nil
}

// abridgeResultForStorage returns a JSON-friendly copy of result with
// the largest per-step debug fields shortened. The history detail page
// still gets useful context (preview text, signature prefix, raw error
// excerpts) without overflowing the column's 64KB ceiling on MySQL.
func abridgeResultForStorage(result *channel_validation.ValidationResult) *channel_validation.ValidationResult {
	if result == nil {
		return nil
	}
	cp := *result
	cp.Steps = make([]channel_validation.StepOutcome, len(result.Steps))
	for i, step := range result.Steps {
		s := step
		s.RawExcerpt = truncateUTF8(s.RawExcerpt, 2048)
		s.ThinkingFull = truncateUTF8(s.ThinkingFull, 4096)
		s.SignatureFull = truncateUTF8(s.SignatureFull, 1024)
		s.OutputTextPreview = truncateUTF8(s.OutputTextPreview, 2048)
		cp.Steps[i] = s
	}
	return &cp
}

// truncateUTF8 cuts s to at most maxBytes UTF-8 bytes, breaking on a
// rune boundary so the stored string remains valid UTF-8.
func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 {
		b := s[cut-1]
		// Walk back into the previous rune.
		if b < 0x80 || b >= 0xC0 {
			break
		}
		cut--
	}
	if cut <= 0 {
		return ""
	}
	return s[:cut] + "…"
}
