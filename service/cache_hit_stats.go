package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func RecordCacheHitStats(userId, channelId int, modelName string, cacheRead, input, cacheWrite int) {
	if cacheRead == 0 && input == 0 && cacheWrite == 0 {
		return
	}
	hour := common.GetTimestamp() - common.GetTimestamp()%3600
	model.LogCacheHitStats(userId, channelId, modelName, hour, cacheRead, input, cacheWrite)
}
