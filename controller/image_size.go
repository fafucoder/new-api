package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
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

	// 更新内存配置（确保线程安全）
	image_size_setting.UpdateSetting(req)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgOperationSuccess),
	})
}

// GetImageSizeValidationSetting 获取图片尺寸硬性校验设置
func GetImageSizeValidationSetting(c *gin.Context) {
	setting := image_size_setting.GetValidationSetting()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    setting,
	})
}

// UpdateImageSizeValidationSetting 更新图片尺寸硬性校验设置
func UpdateImageSizeValidationSetting(c *gin.Context) {
	var req image_size_setting.ImageSizeValidationSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": err.Error()}),
		})
		return
	}

	// 基本参数校验
	if req.MultipleOf <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": "multiple_of must be positive"}),
		})
		return
	}
	if req.MaxEdge <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": "max_edge must be positive"}),
		})
		return
	}
	if req.MaxAspectRatio < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": "max_aspect_ratio must be greater than or equal to 1"}),
		})
		return
	}
	if req.MinPixels < 0 || req.MaxPixels < 0 || (req.MaxPixels > 0 && req.MaxPixels < req.MinPixels) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgInvalidParams, map[string]any{"Error": "invalid pixel range"}),
		})
		return
	}
	if req.Models == nil {
		req.Models = []string{}
	}

	// 序列化模型列表为 JSON 字符串以持久化（切片类型统一以 JSON 存储）
	modelsJSON, err := common.Marshal(req.Models)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgOperationFailed, map[string]any{"Error": err.Error()}),
		})
		return
	}

	updates := []struct {
		key   string
		value string
	}{
		{"image_size_validation.enabled", strconv.FormatBool(req.Enabled)},
		{"image_size_validation.models", string(modelsJSON)},
		{"image_size_validation.multiple_of", strconv.Itoa(req.MultipleOf)},
		{"image_size_validation.max_edge", strconv.Itoa(req.MaxEdge)},
		{"image_size_validation.max_aspect_ratio", strconv.FormatFloat(req.MaxAspectRatio, 'g', -1, 64)},
		{"image_size_validation.min_pixels", strconv.Itoa(req.MinPixels)},
		{"image_size_validation.max_pixels", strconv.Itoa(req.MaxPixels)},
	}
	for _, u := range updates {
		if err := model.UpdateOption(u.key, u.value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgOperationFailed, map[string]any{"Error": err.Error()}),
			})
			return
		}
	}

	// UpdateOption 通过 handleConfigUpdate 反射更新配置，但绕过了 mutex，
	// 这里再显式写入一次以确保线程安全的内存同步。
	image_size_setting.UpdateValidationSetting(req)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgOperationSuccess),
	})
}
