package image_size_setting

import (
	"fmt"
	"strconv"
	"strings"
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

// ImageSizeValidationSetting 图片尺寸硬性校验配置
// 针对特定模型（如 gpt-image-2）对请求的 size 参数做硬性尺寸规则校验，
// 校验失败直接拦截，不转发上游。
type ImageSizeValidationSetting struct {
	Enabled        bool     `json:"enabled"`          // 总开关，默认关闭
	Models         []string `json:"models"`           // 需要校验的模型列表，如 ["gpt-image-2"]
	MultipleOf     int      `json:"multiple_of"`      // 宽、高必须为该值的整数倍
	MaxEdge        int      `json:"max_edge"`         // 单边最大像素
	MaxAspectRatio float64  `json:"max_aspect_ratio"` // 长边/短边 比例上限
	MinPixels      int      `json:"min_pixels"`       // 总像素下限
	MaxPixels      int      `json:"max_pixels"`       // 总像素上限
}

// 默认配置（对齐 gpt-image-2 的硬性尺寸规则）
var defaultImageSizeValidationSetting = ImageSizeValidationSetting{
	Enabled:        false,
	Models:         []string{"gpt-image-2"},
	MultipleOf:     16,
	MaxEdge:        3840,
	MaxAspectRatio: 3.0,
	MinPixels:      655360,
	MaxPixels:      8294400,
}

var (
	imageSizeValidationSetting = defaultImageSizeValidationSetting
	validationMutex            sync.RWMutex
)

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("image_size_validation", &imageSizeValidationSetting)
}

// GetValidationSetting 获取硬性校验配置（返回副本以保证线程安全）
func GetValidationSetting() *ImageSizeValidationSetting {
	validationMutex.RLock()
	defer validationMutex.RUnlock()
	setting := imageSizeValidationSetting
	// 深拷贝切片，避免调用方修改影响全局配置
	if setting.Models != nil {
		models := make([]string, len(setting.Models))
		copy(models, setting.Models)
		setting.Models = models
	}
	return &setting
}

// UpdateValidationSetting 更新硬性校验配置
func UpdateValidationSetting(newSetting ImageSizeValidationSetting) {
	validationMutex.Lock()
	defer validationMutex.Unlock()
	imageSizeValidationSetting = newSetting
}

// ShouldValidateModel 判断某模型是否需要做硬性尺寸校验。
// 当且仅当总开关开启且该模型在校验列表内时返回 true。
func ShouldValidateModel(model string) bool {
	if model == "" {
		return false
	}
	validationMutex.RLock()
	defer validationMutex.RUnlock()
	if !imageSizeValidationSetting.Enabled {
		return false
	}
	for _, m := range imageSizeValidationSetting.Models {
		if m == model {
			return true
		}
	}
	return false
}

// ValidateSize 对给定 size 字符串做硬性尺寸校验。
// - size 为空或 "auto" 时跳过校验（交给上游处理），返回 nil。
// - size 无法解析为 "宽x高" 时跳过校验（保留既有行为，不由本函数负责格式校验），返回 nil。
// - 命中任一硬性规则时返回对应错误，错误文案对齐上游风格。
func ValidateSize(size string) error {
	trimmed := strings.TrimSpace(size)
	if trimmed == "" || strings.EqualFold(trimmed, "auto") {
		return nil
	}

	width, height, ok := parseSize(trimmed)
	if !ok {
		return nil
	}

	validationMutex.RLock()
	multipleOf := imageSizeValidationSetting.MultipleOf
	maxEdge := imageSizeValidationSetting.MaxEdge
	maxAspectRatio := imageSizeValidationSetting.MaxAspectRatio
	minPixels := imageSizeValidationSetting.MinPixels
	maxPixels := imageSizeValidationSetting.MaxPixels
	validationMutex.RUnlock()

	longEdge, shortEdge := width, height
	if height > width {
		longEdge, shortEdge = height, width
	}

	// 规则2：单边最大
	if maxEdge > 0 && longEdge > maxEdge {
		return fmt.Errorf("Invalid size '%s'. The longest edge must be less than or equal to %d.", size, maxEdge)
	}

	// 规则1：宽、高均为指定值的整数倍
	if multipleOf > 0 && (width%multipleOf != 0 || height%multipleOf != 0) {
		return fmt.Errorf("Invalid size '%s'. Both width and height must be multiples of %d.", size, multipleOf)
	}

	// 规则3：长边/短边 比例上限
	if maxAspectRatio > 0 && shortEdge > 0 && float64(longEdge) > float64(shortEdge)*maxAspectRatio {
		return fmt.Errorf("Invalid size '%s'. The ratio of the longest edge to the shortest edge must be less than or equal to %s:1.", size, strconv.FormatFloat(maxAspectRatio, 'g', -1, 64))
	}

	// 规则4：总像素范围
	totalPixels := width * height
	if (minPixels > 0 && totalPixels < minPixels) || (maxPixels > 0 && totalPixels > maxPixels) {
		return fmt.Errorf("Invalid size '%s'. The total number of pixels must be between %d and %d.", size, minPixels, maxPixels)
	}

	return nil
}

// parseSize 解析 "宽x高" 格式（分隔符 x/X），成功返回正整数宽高。
func parseSize(size string) (int, int, bool) {
	sep := "x"
	if !strings.Contains(size, "x") && strings.Contains(size, "X") {
		sep = "X"
	}
	parts := strings.SplitN(size, sep, 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
