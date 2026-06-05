package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/image_size_setting"
	"github.com/gin-gonic/gin"
)

// GetImageSizeSetting 获取图片尺寸设置
func GetImageSizeSetting(c *gin.Context) {
	setting := image_size_setting.GetSetting()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    setting,
	})
}

// UpdateImageSizeSetting 更新图片尺寸设置
func UpdateImageSizeSetting(c *gin.Context) {
	var req image_size_setting.ImageSizeSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": err.Error()}),
		})
		return
	}

	// 验证模式
	if req.Mode != image_size_setting.ModeTotalPixels &&
		req.Mode != image_size_setting.ModeMinEdge &&
		req.Mode != image_size_setting.ModeMaxEdge {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": "invalid mode"}),
		})
		return
	}

	// 验证阈值
	if req.ThresholdValue <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": "threshold_value must be positive"}),
		})
		return
	}

	// 保存到数据库
	modeValue := string(req.Mode)
	thresholdValue := strconv.Itoa(req.ThresholdValue)

	if err := model.UpdateOption("image_size_setting.mode", modeValue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOperationFailed, map[string]any{"Error": err.Error()}),
		})
		return
	}

	if err := model.UpdateOption("image_size_setting.threshold_value", thresholdValue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOperationFailed, map[string]any{"Error": err.Error()}),
		})
		return
	}

	// 虽然 model.UpdateOption 通过 handleConfigUpdate -> UpdateConfigFromMap 更新了配置，
	// 但 UpdateConfigFromMap 直接通过反射修改，没有使用 image_size_setting 的 mutex 保护。
	// 必须调用 UpdateSetting 以确保线程安全的更新和正确的内存同步。
	image_size_setting.UpdateSetting(req)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgOperationSuccess),
	})
}
