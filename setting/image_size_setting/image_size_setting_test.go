package image_size_setting

import (
	"strings"
	"testing"
)

func TestIsHighResolution_TotalPixels(t *testing.T) {
	tests := []struct {
		name      string
		mode      ImageSizeMode
		threshold int
		width     int
		height    int
		want      bool
	}{
		{
			name:      "总像素模式 - 超过阈值（8MP）",
			mode:      ModeTotalPixels,
			threshold: 8_000_000,
			width:     3840,
			height:    2160,
			want:      true, // 3840*2160 = 8,294,400 > 8,000,000
		},
		{
			name:      "总像素模式 - 等于阈值边界",
			mode:      ModeTotalPixels,
			threshold: 8_294_400,
			width:     3840,
			height:    2160,
			want:      false, // 3840*2160 = 8,294,400，不大于阈值
		},
		{
			name:      "总像素模式 - 低于阈值（1024x1024）",
			mode:      ModeTotalPixels,
			threshold: 8_000_000,
			width:     1024,
			height:    1024,
			want:      false, // 1024*1024 = 1,048,576 < 8,000,000
		},
		{
			name:      "总像素模式 - 刚好超过阈值一像素",
			mode:      ModeTotalPixels,
			threshold: 1_000_000,
			width:     1001,
			height:    1000,
			want:      true, // 1,001,000 > 1,000,000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 临时设置配置
			mutex.Lock()
			imageSizeSetting.Mode = tt.mode
			imageSizeSetting.ThresholdValue = tt.threshold
			mutex.Unlock()

			got := IsHighResolution(tt.width, tt.height)
			if got != tt.want {
				t.Errorf("IsHighResolution(%d, %d) = %v, want %v (threshold=%d, mode=%s)",
					tt.width, tt.height, got, tt.want, tt.threshold, tt.mode)
			}
		})
	}
}

func TestIsHighResolution_MinEdge(t *testing.T) {
	tests := []struct {
		name      string
		mode      ImageSizeMode
		threshold int
		width     int
		height    int
		want      bool
	}{
		{
			name:      "最小边模式 - 超过阈值",
			mode:      ModeMinEdge,
			threshold: 1024,
			width:     2048,
			height:    1536,
			want:      true, // min(2048, 1536) = 1536 > 1024
		},
		{
			name:      "最小边模式 - 等于阈值边界",
			mode:      ModeMinEdge,
			threshold: 1024,
			width:     1024,
			height:    2048,
			want:      false, // min(1024, 2048) = 1024，不大于阈值
		},
		{
			name:      "最小边模式 - 低于阈值",
			mode:      ModeMinEdge,
			threshold: 1024,
			width:     800,
			height:    600,
			want:      false, // min(800, 600) = 600 < 1024
		},
		{
			name:      "最小边模式 - 刚好超过阈值一像素",
			mode:      ModeMinEdge,
			threshold: 512,
			width:     513,
			height:    1024,
			want:      true, // min(513, 1024) = 513 > 512
		},
		{
			name:      "最小边模式 - 宽高相等且超过阈值",
			mode:      ModeMinEdge,
			threshold: 1024,
			width:     2048,
			height:    2048,
			want:      true, // min(2048, 2048) = 2048 > 1024
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 临时设置配置
			mutex.Lock()
			imageSizeSetting.Mode = tt.mode
			imageSizeSetting.ThresholdValue = tt.threshold
			mutex.Unlock()

			got := IsHighResolution(tt.width, tt.height)
			if got != tt.want {
				t.Errorf("IsHighResolution(%d, %d) = %v, want %v (threshold=%d, mode=%s)",
					tt.width, tt.height, got, tt.want, tt.threshold, tt.mode)
			}
		})
	}
}

func TestIsHighResolution_MaxEdge(t *testing.T) {
	tests := []struct {
		name      string
		mode      ImageSizeMode
		threshold int
		width     int
		height    int
		want      bool
	}{
		{
			name:      "最大边模式 - 超过阈值",
			mode:      ModeMaxEdge,
			threshold: 1024,
			width:     2048,
			height:    512,
			want:      true, // max(2048, 512) = 2048 > 1024
		},
		{
			name:      "最大边模式 - 等于阈值边界",
			mode:      ModeMaxEdge,
			threshold: 2048,
			width:     1024,
			height:    2048,
			want:      false, // max(1024, 2048) = 2048，不大于阈值
		},
		{
			name:      "最大边模式 - 低于阈值",
			mode:      ModeMaxEdge,
			threshold: 2048,
			width:     1024,
			height:    768,
			want:      false, // max(1024, 768) = 1024 < 2048
		},
		{
			name:      "最大边模式 - 刚好超过阈值一像素",
			mode:      ModeMaxEdge,
			threshold: 1920,
			width:     1080,
			height:    1921,
			want:      true, // max(1080, 1921) = 1921 > 1920
		},
		{
			name:      "最大边模式 - 宽高相等且超过阈值",
			mode:      ModeMaxEdge,
			threshold: 1024,
			width:     2048,
			height:    2048,
			want:      true, // max(2048, 2048) = 2048 > 1024
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 临时设置配置
			mutex.Lock()
			imageSizeSetting.Mode = tt.mode
			imageSizeSetting.ThresholdValue = tt.threshold
			mutex.Unlock()

			got := IsHighResolution(tt.width, tt.height)
			if got != tt.want {
				t.Errorf("IsHighResolution(%d, %d) = %v, want %v (threshold=%d, mode=%s)",
					tt.width, tt.height, got, tt.want, tt.threshold, tt.mode)
			}
		})
	}
}

func setValidationSetting(s ImageSizeValidationSetting) {
	validationMutex.Lock()
	imageSizeValidationSetting = s
	validationMutex.Unlock()
}

// gpt-image-2 硬性规则默认配置
var gptImage2Setting = ImageSizeValidationSetting{
	Enabled:        true,
	Models:         []string{"gpt-image-2"},
	MultipleOf:     16,
	MaxEdge:        3840,
	MaxAspectRatio: 3.0,
	MinPixels:      655360,
	MaxPixels:      8294400,
}

func TestValidateSize(t *testing.T) {
	setValidationSetting(gptImage2Setting)

	tests := []struct {
		name       string
		size       string
		wantErr    bool
		wantSubstr string // 期望错误信息包含的关键片段
	}{
		{name: "合法尺寸 1024x1024", size: "1024x1024", wantErr: false},
		{name: "合法尺寸 3840x2160（4K，比例1.78，像素8294400）", size: "3840x2160", wantErr: false},
		{name: "空 size 跳过", size: "", wantErr: false},
		{name: "auto 跳过", size: "auto", wantErr: false},
		{name: "AUTO 大小写跳过", size: "AUTO", wantErr: false},
		{name: "无法解析跳过", size: "abc", wantErr: false},
		{name: "单边超限 5120x2880", size: "5120x2880", wantErr: true, wantSubstr: "The longest edge must be less than or equal to 3840"},
		{name: "非16倍数 1000x1000", size: "1000x1000", wantErr: true, wantSubstr: "multiples of 16"},
		{name: "比例超3:1 3840x512", size: "3840x512", wantErr: true, wantSubstr: "ratio of the longest edge"},
		{name: "像素低于下限 512x512", size: "512x512", wantErr: true, wantSubstr: "total number of pixels"},
		{name: "X 大写分隔符 1024X1024", size: "1024X1024", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSize(tt.size)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateSize(%q) 期望报错，实际为 nil", tt.size)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateSize(%q) 期望无错，实际: %v", tt.size, err)
			}
			if tt.wantErr && tt.wantSubstr != "" && !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Errorf("ValidateSize(%q) 错误信息 = %q，期望包含 %q", tt.size, err.Error(), tt.wantSubstr)
			}
		})
	}
}

func TestValidateSize_SingleEdgeCheckedBeforeMultiple(t *testing.T) {
	// 5120x2880: 既超单边(5120>3840)，又是16倍数校验之前应先报单边超限
	setValidationSetting(gptImage2Setting)
	err := ValidateSize("5120x2880")
	if err == nil || !strings.Contains(err.Error(), "longest edge") {
		t.Errorf("期望优先报单边超限，实际: %v", err)
	}
}

func TestShouldValidateModel(t *testing.T) {
	setValidationSetting(gptImage2Setting)
	if !ShouldValidateModel("gpt-image-2") {
		t.Error("gpt-image-2 应需要校验")
	}
	if ShouldValidateModel("gpt-image-1") {
		t.Error("gpt-image-1 不在列表，不应校验")
	}
	if ShouldValidateModel("") {
		t.Error("空模型名不应校验")
	}

	// 关闭总开关后一律不校验
	disabled := gptImage2Setting
	disabled.Enabled = false
	setValidationSetting(disabled)
	if ShouldValidateModel("gpt-image-2") {
		t.Error("总开关关闭时不应校验")
	}
}

func TestGetValidationSetting_ReturnsCopy(t *testing.T) {
	setValidationSetting(gptImage2Setting)
	got := GetValidationSetting()
	got.Models[0] = "mutated"
	// 再取一次，确认全局配置未被修改
	again := GetValidationSetting()
	if again.Models[0] != "gpt-image-2" {
		t.Errorf("GetValidationSetting 应返回深拷贝，全局被污染: %v", again.Models)
	}
}
