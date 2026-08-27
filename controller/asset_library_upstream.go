package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// asset library upstream admin handlers. Upstream configuration (base URL, API
// key, format, actions) is managed here, decoupled from relay channels.

type assetLibraryUpstreamRequest struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Enabled *bool  `json:"enabled"`

	Version           string `json:"version"`
	ProjectName       string `json:"project_name"`
	ListGroupsAction  string `json:"list_groups_action"`
	CreateGroupAction string `json:"create_group_action"`
	GetGroupAction    string `json:"get_group_action"`
	CreateAssetAction string `json:"create_asset_action"`
	GetAssetAction    string `json:"get_asset_action"`
	UpdateAssetAction string `json:"update_asset_action"`
	DeleteAssetAction string `json:"delete_asset_action"`

	Purpose string `json:"purpose"`
}

// sanitizedUpstream hides the API key when returning upstreams to the admin UI.
func sanitizedUpstream(upstream model.AssetLibraryUpstream) model.AssetLibraryUpstream {
	if upstream.APIKey != "" {
		upstream.APIKey = ""
	}
	return upstream
}

func GetAssetLibraryUpstreams(c *gin.Context) {
	upstreams, err := model.ListAssetLibraryUpstreams()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	safe := make([]model.AssetLibraryUpstream, 0, len(upstreams))
	for _, upstream := range upstreams {
		safe = append(safe, sanitizedUpstream(upstream))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": safe})
}

func validateAssetLibraryUpstreamRequest(request *assetLibraryUpstreamRequest) (string, string, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return "", "", errAssetUpstream("name is required")
	}
	format := strings.TrimSpace(request.Format)
	if format == "" {
		format = model.AssetLibraryFormatVolcengine
	}
	if format != model.AssetLibraryFormatVolcengine && format != model.AssetLibraryFormatOpenAI {
		return "", "", errAssetUpstream("format must be volcengine or openai")
	}
	if strings.TrimSpace(request.BaseURL) == "" {
		return "", "", errAssetUpstream("base_url is required")
	}
	return name, format, nil
}

func errAssetUpstream(message string) error {
	return &assetUpstreamError{message: message}
}

type assetUpstreamError struct{ message string }

func (e *assetUpstreamError) Error() string { return e.message }

func CreateAssetLibraryUpstream(c *gin.Context) {
	var request assetLibraryUpstreamRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	name, format, err := validateAssetLibraryUpstreamRequest(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	upstream := &model.AssetLibraryUpstream{
		Name:              name,
		Format:            format,
		BaseURL:           strings.TrimSpace(request.BaseURL),
		APIKey:            strings.TrimSpace(request.APIKey),
		Enabled:           enabled,
		Version:           strings.TrimSpace(request.Version),
		ProjectName:       strings.TrimSpace(request.ProjectName),
		ListGroupsAction:  strings.TrimSpace(request.ListGroupsAction),
		CreateGroupAction: strings.TrimSpace(request.CreateGroupAction),
		GetGroupAction:    strings.TrimSpace(request.GetGroupAction),
		CreateAssetAction: strings.TrimSpace(request.CreateAssetAction),
		GetAssetAction:    strings.TrimSpace(request.GetAssetAction),
		UpdateAssetAction: strings.TrimSpace(request.UpdateAssetAction),
		DeleteAssetAction: strings.TrimSpace(request.DeleteAssetAction),
		Purpose:           strings.TrimSpace(request.Purpose),
	}
	if err := model.CreateAssetLibraryUpstream(upstream); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sanitizedUpstream(*upstream)})
}

func UpdateAssetLibraryUpstream(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid upstream id"})
		return
	}
	var request assetLibraryUpstreamRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	name, format, err := validateAssetLibraryUpstreamRequest(&request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	values := map[string]interface{}{
		"name":                name,
		"format":              format,
		"base_url":            strings.TrimSpace(request.BaseURL),
		"version":             strings.TrimSpace(request.Version),
		"project_name":        strings.TrimSpace(request.ProjectName),
		"list_groups_action":  strings.TrimSpace(request.ListGroupsAction),
		"create_group_action": strings.TrimSpace(request.CreateGroupAction),
		"get_group_action":    strings.TrimSpace(request.GetGroupAction),
		"create_asset_action": strings.TrimSpace(request.CreateAssetAction),
		"get_asset_action":    strings.TrimSpace(request.GetAssetAction),
		"update_asset_action": strings.TrimSpace(request.UpdateAssetAction),
		"delete_asset_action": strings.TrimSpace(request.DeleteAssetAction),
		"purpose":             strings.TrimSpace(request.Purpose),
	}
	if request.Enabled != nil {
		values["enabled"] = *request.Enabled
	}
	// Only overwrite the API key when a new non-empty value is provided, so the
	// admin UI can omit it to keep the stored secret.
	if strings.TrimSpace(request.APIKey) != "" {
		values["api_key"] = strings.TrimSpace(request.APIKey)
	}
	if err := model.UpdateAssetLibraryUpstream(id, values); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "upstream not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	upstream, err := model.GetAssetLibraryUpstream(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sanitizedUpstream(*upstream)})
}

func DeleteAssetLibraryUpstream(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid upstream id"})
		return
	}
	if err := model.DeleteAssetLibraryUpstream(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "upstream not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
