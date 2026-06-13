package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 清空 logs，保证用例间隔离
func resetLogs(t *testing.T) {
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
}

// 插入一条指定类型/渠道/用户/时间的日志
func mkLog(t *testing.T, logType, channelId, userId int, createdAt int64) {
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    userId,
		CreatedAt: createdAt,
		Type:      logType,
		ChannelId: channelId,
	}).Error)
}

func TestCalcErrorRate(t *testing.T) {
	assert.Equal(t, 0.0, calcErrorRate(0, 0))
	assert.Equal(t, 30.0, calcErrorRate(3, 7))
	assert.Equal(t, 33.33, calcErrorRate(1, 2))
	assert.Equal(t, 100.0, calcErrorRate(5, 0))
}

func TestQueryErrorRate_Basic(t *testing.T) {
	resetLogs(t)
	// 窗口 [1000, 1000+300*12)，bucketSize=300
	var start int64 = 1200
	var bucketSize int64 = 300
	var end = start + bucketSize*11 + 299 // 落在第 12 桶内
	// 第一个桶内：3 错误 + 7 成功
	for i := 0; i < 3; i++ {
		mkLog(t, LogTypeError, 1, 1, start+10)
	}
	for i := 0; i < 7; i++ {
		mkLog(t, LogTypeConsume, 1, 1, start+20)
	}
	res, err := QueryErrorRate(0, nil, false, "1h", start, end, bucketSize)
	require.NoError(t, err)
	assert.Equal(t, int64(3), res.ErrorCount)
	assert.Equal(t, int64(7), res.SuccessCount)
	assert.Equal(t, int64(10), res.Total)
	assert.Equal(t, 30.0, res.ErrorRate)
	assert.Equal(t, 12, len(res.Trend)) // 桶数恒定
	assert.Equal(t, 30.0, res.Trend[0].ErrorRate)
	assert.Equal(t, 0.0, res.Trend[1].ErrorRate)
}

func TestQueryErrorRate_TotalZero(t *testing.T) {
	resetLogs(t)
	var start, bucketSize int64 = 1200, 300
	end := start + bucketSize*11
	res, err := QueryErrorRate(0, nil, false, "1h", start, end, bucketSize)
	require.NoError(t, err)
	assert.Equal(t, int64(0), res.Total)
	assert.Equal(t, 0.0, res.ErrorRate)
	assert.Equal(t, 12, len(res.Trend))
	for _, b := range res.Trend {
		assert.Equal(t, 0.0, b.ErrorRate)
		assert.Equal(t, int64(0), b.ErrorCount)
	}
}

func TestQueryErrorRate_OutOfWindow(t *testing.T) {
	resetLogs(t)
	var start, bucketSize int64 = 1200, 300
	end := start + bucketSize*11
	mkLog(t, LogTypeError, 1, 1, start-50)  // 窗口前
	mkLog(t, LogTypeError, 1, 1, end+5000)  // 窗口后
	mkLog(t, LogTypeConsume, 1, 1, start+5) // 窗口内
	res, err := QueryErrorRate(0, nil, false, "1h", start, end, bucketSize)
	require.NoError(t, err)
	assert.Equal(t, int64(0), res.ErrorCount)
	assert.Equal(t, int64(1), res.SuccessCount)
}

func TestQueryErrorRate_UserFilter(t *testing.T) {
	resetLogs(t)
	var start, bucketSize int64 = 1200, 300
	end := start + bucketSize*11
	mkLog(t, LogTypeError, 1, 1, start+5)   // user 1
	mkLog(t, LogTypeConsume, 1, 2, start+5) // user 2
	res, err := QueryErrorRate(1, nil, false, "1h", start, end, bucketSize)
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.ErrorCount)
	assert.Equal(t, int64(0), res.SuccessCount)
}

func TestQueryErrorRate_ChannelFilter(t *testing.T) {
	resetLogs(t)
	var start, bucketSize int64 = 1200, 300
	end := start + bucketSize*11
	mkLog(t, LogTypeError, 10, 1, start+5)
	mkLog(t, LogTypeConsume, 20, 1, start+5)
	res, err := QueryErrorRate(0, []int{10}, true, "1h", start, end, bucketSize)
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.ErrorCount)
	assert.Equal(t, int64(0), res.SuccessCount)
}

func TestQueryErrorRate_EmptyChannelIDs(t *testing.T) {
	resetLogs(t)
	var start, bucketSize int64 = 1200, 300
	end := start + bucketSize*11
	mkLog(t, LogTypeError, 10, 1, start+5)
	// applyChannelFilter=true 但列表空 → 命中 0 行
	res, err := QueryErrorRate(0, []int{}, true, "1h", start, end, bucketSize)
	require.NoError(t, err)
	assert.Equal(t, int64(0), res.Total)
}

func TestQueryErrorRate_Buckets(t *testing.T) {
	resetLogs(t)
	var start, bucketSize int64 = 1200, 300
	end := start + bucketSize*11
	mkLog(t, LogTypeError, 1, 1, start+5)              // 桶0
	mkLog(t, LogTypeConsume, 1, 1, start+bucketSize+5) // 桶1
	res, err := QueryErrorRate(0, nil, false, "1h", start, end, bucketSize)
	require.NoError(t, err)
	assert.Equal(t, 100.0, res.Trend[0].ErrorRate)
	assert.Equal(t, 0.0, res.Trend[1].ErrorRate)
	assert.Equal(t, start, res.Trend[0].Time)
	assert.Equal(t, start+bucketSize, res.Trend[1].Time)
}
