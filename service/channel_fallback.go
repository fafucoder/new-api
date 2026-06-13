package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// GetFallbackChannel 获取支持指定模型的兜底渠道
// 返回按优先级/权重规则选择的最优兜底渠道
func GetFallbackChannel(modelName string, usedChannelIDs []string) (*model.Channel, error) {
	db := model.DB.Where("is_fallback = ?", true).
		Where("status = ?", common.ChannelStatusEnabled)

	// 模型匹配（严格匹配）
	db = db.Where("FIND_IN_SET(?, models) > 0", modelName)

	// 排除已使用的渠道（去重）
	if len(usedChannelIDs) > 0 {
		// 将字符串 ID 转换为整数 ID
		var intIDs []int
		for _, idStr := range usedChannelIDs {
			// 移除 "fallback:" 前缀（如果有）
			idStr = strings.TrimPrefix(idStr, "fallback:")
			if id, err := strconv.Atoi(idStr); err == nil {
				intIDs = append(intIDs, id)
			}
		}
		if len(intIDs) > 0 {
			db = db.Where("id NOT IN (?)", intIDs)
		}
	}

	// 排序（优先级、权重）
	db = db.Order("priority DESC").Order("weight DESC")

	// 获取第一个匹配的渠道
	var channel model.Channel
	if err := db.First(&channel).Error; err != nil {
		return nil, fmt.Errorf("no fallback channel available for model %s: %w", modelName, err)
	}

	return &channel, nil
}
