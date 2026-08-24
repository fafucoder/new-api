package model

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeChannelCostQuota 验证进货价计算：成本 = 标准价 × channel_ratio，
// 不应受分组折扣影响（售价已含 group_ratio，需先除掉再乘 channel_ratio）。
func TestComputeChannelCostQuota(t *testing.T) {
	tests := []struct {
		name         string
		quota        int
		channelRatio float64
		groupRatio   float64
		want         int
	}{
		{
			// 核心回归：kimi 场景。售价 quota=600（=标准价1000×分组0.6），
			// 渠道倍率 0.5 → 成本 = 600/0.6*0.5 = 500，利润率 (600-500)/500 = 20%。
			name:         "group discount excluded from cost",
			quota:        600,
			channelRatio: 0.5,
			groupRatio:   0.6,
			want:         500,
		},
		{
			// 无分组折扣时，成本 = 售价 × 渠道倍率（口径不变）。
			name:         "group ratio 1.0 unchanged",
			quota:        1000,
			channelRatio: 0.5,
			groupRatio:   1.0,
			want:         500,
		},
		{
			// group_ratio<=0（免费组/未提供快照）退回旧口径 cost=quota*channelRatio。
			name:         "zero group ratio falls back",
			quota:        1000,
			channelRatio: 0.5,
			groupRatio:   0,
			want:         500,
		},
		{
			// 免费组 quota 本就是 0，退回旧口径结果仍为 0。
			name:         "free model zero quota",
			quota:        0,
			channelRatio: 0.5,
			groupRatio:   0,
			want:         0,
		},
		{
			// channel_ratio=1.0 且分组倍率 0.7：成本 = 700/0.7*1.0 = 1000（还原标准价）。
			name:         "channel ratio 1.0 restores standard price",
			quota:        700,
			channelRatio: 1.0,
			groupRatio:   0.7,
			want:         1000,
		},
		{
			// 负 group_ratio 视为无效，退回旧口径（防御性）。
			name:         "negative group ratio falls back",
			quota:        1000,
			channelRatio: 0.5,
			groupRatio:   -1,
			want:         500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeChannelCostQuota(tt.quota, tt.channelRatio, tt.groupRatio)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestComputeChannelCostQuotaSaturation 验证越界乘积被饱和到 MaxQuota，绝不回绕成负数。
func TestComputeChannelCostQuotaSaturation(t *testing.T) {
	got := computeChannelCostQuota(math.MaxInt32, 100, 0.0001)
	// 饱和上界为 int32 最大值（quota 列是 32 位），绝不回绕成负数。
	assert.Equal(t, math.MaxInt32, got)
	assert.GreaterOrEqual(t, got, 0)
}
