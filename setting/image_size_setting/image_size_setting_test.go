package image_size_setting

import (
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
