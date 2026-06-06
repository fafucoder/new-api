package image_size_setting

import (
	"sync"

	"github.com/QuantumNous/new-api/setting/config"
)

// ImageSizeMode 图片尺寸判断模式
type ImageSizeMode string

const (
	ModeTotalPixels ImageSizeMode = "total_pixels" // 总像素数模式
	ModeMinEdge     ImageSizeMode = "min_edge"     // 最小边模式
	ModeMaxEdge     ImageSizeMode = "max_edge"     // 最大边模式
)

// ImageSizeSetting 图片尺寸设置
type ImageSizeSetting struct {
	Mode           ImageSizeMode `json:"mode"`            // 判断模式: total_pixels, min_edge, max_edge
	ThresholdValue int           `json:"threshold_value"` // 阈值
}

// 默认配置
var defaultImageSizeSetting = ImageSizeSetting{
	Mode:           ModeMinEdge, // 默认使用最小边模式
	ThresholdValue: 1024,        // 默认阈值 1024
}

// 全局实例
var (
	imageSizeSetting = defaultImageSizeSetting
	mutex            sync.RWMutex
)

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("image_size_setting", &imageSizeSetting)
}

// GetSetting 获取 ImageSizeSetting 配置实例（返回副本以保证线程安全）
func GetSetting() *ImageSizeSetting {
	mutex.RLock()
	defer mutex.RUnlock()
	setting := imageSizeSetting
	return &setting
}

// UpdateSetting 更新 ImageSizeSetting 配置
func UpdateSetting(newSetting ImageSizeSetting) {
	mutex.Lock()
	defer mutex.Unlock()
	imageSizeSetting = newSetting
}

// IsHighResolution 判断图片是否为高分辨率
func IsHighResolution(width, height int) bool {
	// 边界值校验
	if width <= 0 || height <= 0 {
		return false
	}

	mutex.RLock()
	mode := imageSizeSetting.Mode
	threshold := imageSizeSetting.ThresholdValue
	mutex.RUnlock()

	switch mode {
	case ModeTotalPixels:
		// 总像素数模式：宽度 × 高度 > 阈值
		return width*height > threshold
	case ModeMinEdge:
		// 最小边模式：min(宽度, 高度) > 阈值
		minEdge := width
		if height < minEdge {
			minEdge = height
		}
		return minEdge > threshold
	case ModeMaxEdge:
		// 最大边模式：max(宽度, 高度) > 阈值
		maxEdge := width
		if height > maxEdge {
			maxEdge = height
		}
		return maxEdge > threshold
	default:
		// 未知模式，默认返回 false
		return false
	}
}
