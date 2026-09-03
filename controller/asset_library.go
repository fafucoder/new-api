package controller

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const assetLibraryMaxFileSize = int64(200 << 20)
const assetLibraryMaxFileCount = int64(20)
const assetLibraryMultipartOverhead = int64(20 << 20)

type updateAssetLibraryGroupRequest struct {
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

type updateAssetLibraryAssetRequest struct {
	Name string `json:"name"`
}

type createAssetLibraryGroupRequest struct {
	DisplayName string `json:"display_name"`
	GroupType   string `json:"group_type"`
	Description string `json:"description"`
}

type appendAssetLibraryURLsRequest struct {
	Assets []service.AssetURLInput `json:"assets"`
}

func ensureAssetLibraryEnabled(c *gin.Context) bool {
	if isAssetLibraryEnabled() {
		return true
	}
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset library is disabled"})
	return false
}

// isAssetLibraryEnabled is intentionally shared by the status endpoint and all
// asset-library handlers so the navigation state cannot drift from API access.
// SidebarModulesAdmin is the source of truth after the setting was moved from
// Operations to the admin sidebar configuration. The legacy option is only
// used when no sidebar configuration has ever been saved.
func isAssetLibraryEnabled() bool {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap["SidebarModulesAdmin"]
	common.OptionMapRWMutex.RUnlock()

	if strings.TrimSpace(raw) != "" {
		var config map[string]map[string]bool
		if err := common.UnmarshalJsonStr(raw, &config); err == nil {
			console, ok := config["console"]
			if !ok {
				return false
			}
			return console["enabled"] && console["assets"]
		}
		return false
	}

	// Existing installations may not have SidebarModulesAdmin yet. Preserve
	// their previous choice until an administrator saves the new setting.
	return operation_setting.AssetLibraryEnabled
}

func GetAssetLibraryChannels(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	upstreams, err := service.ListAssetLibraryUpstreamsForUser()
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": upstreams})
}

func GetAssetLibraryGroups(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groups, err := service.ListAssetLibraryGroups(c.GetInt("id"))
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": groups})
}

func GetAssetLibraryGroup(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groupId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset group id"})
		return
	}
	group, err := service.GetAssetLibraryGroup(c.GetInt("id"), groupId)
	if err != nil {
		assetLibraryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": group})
}

func PostAssetLibraryGroup(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	var request createAssetLibraryGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	group, results, err := service.CreateAssetLibraryGroup(
		c.Request.Context(), c.GetInt("id"), strings.TrimSpace(request.DisplayName), strings.TrimSpace(request.GroupType), strings.TrimSpace(request.Description),
	)
	if err != nil {
		assetLibraryErrorWithData(c, err, results)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"group": group, "results": results}})
}

func PostAssetLibraryGroupAssets(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groupId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset group id"})
		return
	}

	// A JSON body imports public URLs; a multipart body uploads local files.
	if strings.HasPrefix(c.ContentType(), "application/json") {
		var request appendAssetLibraryURLsRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
			return
		}
		group, results, err := service.AppendAssetLibraryURLs(c.Request.Context(), c.GetInt("id"), groupId, request.Assets)
		if err != nil {
			assetLibraryErrorWithData(c, err, results)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"group": group, "results": results}})
		return
	}

	files, cleanup, ok := assetLibraryFiles(c)
	if !ok {
		return
	}
	defer cleanup()
	group, results, err := service.AppendAssetLibraryFiles(c.Request.Context(), c.GetInt("id"), groupId, files)
	if err != nil {
		assetLibraryErrorWithData(c, err, results)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"group": group, "results": results}})
}

func PatchAssetLibraryGroup(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groupId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset group id"})
		return
	}
	var request updateAssetLibraryGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	group, results, err := service.UpdateAssetLibraryGroup(c.Request.Context(), c.GetInt("id"), groupId, request.DisplayName, request.Description)
	if err != nil {
		assetLibraryErrorWithData(c, err, results)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"group": group, "results": results}})
}

func PostRefreshAssetLibraryGroup(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groupId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset group id"})
		return
	}
	group, results, err := service.RefreshAssetLibraryGroup(c.Request.Context(), c.GetInt("id"), groupId)
	if err != nil {
		assetLibraryErrorWithData(c, err, results)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"group": group, "results": results}})
}

func PatchAssetLibraryAsset(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groupId, groupErr := strconv.ParseInt(c.Param("id"), 10, 64)
	assetId, assetErr := strconv.ParseInt(c.Param("assetId"), 10, 64)
	if groupErr != nil || assetErr != nil || groupId <= 0 || assetId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset id"})
		return
	}
	var request updateAssetLibraryAssetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	group, results, err := service.UpdateAssetLibraryAsset(c.Request.Context(), c.GetInt("id"), groupId, assetId, request.Name)
	if err != nil {
		assetLibraryErrorWithData(c, err, results)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"group": group, "results": results}})
}

func DeleteAssetLibraryGroup(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groupId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset group id"})
		return
	}
	results, err := service.DeleteAssetLibraryGroup(c.Request.Context(), c.GetInt("id"), groupId, assetLibraryForce(c))
	if err != nil {
		assetLibraryErrorWithData(c, err, results)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func DeleteAssetLibraryAsset(c *gin.Context) {
	if !ensureAssetLibraryEnabled(c) {
		return
	}
	groupId, groupErr := strconv.ParseInt(c.Param("id"), 10, 64)
	assetId, assetErr := strconv.ParseInt(c.Param("assetId"), 10, 64)
	if groupErr != nil || assetErr != nil || groupId <= 0 || assetId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset id"})
		return
	}
	results, err := service.DeleteAssetLibraryAsset(c.Request.Context(), c.GetInt("id"), groupId, assetId, assetLibraryForce(c))
	if err != nil {
		assetLibraryErrorWithData(c, err, results)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

func assetLibraryFiles(c *gin.Context) ([]*multipart.FileHeader, func(), bool) {
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		assetLibraryMaxFileSize*assetLibraryMaxFileCount+assetLibraryMultipartOverhead,
	)
	form, err := c.MultipartForm()
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "asset upload request is too large"})
			return nil, nil, false
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid multipart form"})
		return nil, nil, false
	}
	cleanup := func() { _ = form.RemoveAll() }
	files := form.File["files"]
	if len(files) == 0 {
		cleanup()
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "at least one file is required"})
		return nil, nil, false
	}
	if int64(len(files)) > assetLibraryMaxFileCount {
		cleanup()
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "a maximum of 20 files can be uploaded at once"})
		return nil, nil, false
	}
	for _, file := range files {
		if file.Size > assetLibraryMaxFileSize {
			cleanup()
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "each file must not exceed 200 MiB"})
			return nil, nil, false
		}
	}
	return files, cleanup, true
}

func assetLibraryError(c *gin.Context, err error) {
	assetLibraryErrorWithData(c, err, nil)
}

// assetLibraryForce reports whether the caller requested a force delete, which
// removes the local record even if some upstreams fail to delete.
func assetLibraryForce(c *gin.Context) bool {
	value := strings.ToLower(strings.TrimSpace(c.Query("force")))
	return value == "1" || value == "true"
}

func assetLibraryErrorWithData(c *gin.Context, err error, data interface{}) {
	status := http.StatusInternalServerError
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status = http.StatusNotFound
	} else if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "maximum") || strings.Contains(err.Error(), "must") || strings.Contains(err.Error(), "configured") {
		status = http.StatusBadRequest
	} else if strings.Contains(err.Error(), "upstream") {
		status = http.StatusBadGateway
	}
	c.JSON(status, gin.H{"success": false, "message": err.Error(), "data": data})
}
