package model

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 清空 logs，保证用例间隔离
func resetLogs(t *testing.T) {
	require.NoError(t, LOG_DB.Exec("DELETE FROM logs").Error)
}

var reqIDSeq uint64

// mkLog 插入一条指定类型/渠道/用户/时间的日志，自动生成唯一 request_id。
// QueryErrorRate 使用 `request_id != ''` 过滤无 request_id 的日志，所以测试桩必须填充。
func mkLog(t *testing.T, logType, channelId, userId int, createdAt int64) {
	id := atomic.AddUint64(&reqIDSeq, 1)
	mkLogWithRID(t, logType, channelId, userId, createdAt, fmt.Sprintf("req-%d", id))
}

// mkLogWithRID 以指定 request_id 插入日志，用于模拟同一请求的多条日志（重试场景）。
func mkLogWithRID(t *testing.T, logType, channelId, userId int, createdAt int64, requestId string) {
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    userId,
		CreatedAt: createdAt,
		Type:      logType,
		ChannelId: channelId,
		RequestId: requestId,
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

// 复现「错误率与下方指标对不上」根因：同一 request_id 的失败和最终成功若跨桶，
// 不能被分别记为 1 次失败 + 1 次成功，应只算最终结果，并归入首次请求所在的桶。
func TestQueryErrorRate_RetryAcrossBuckets(t *testing.T) {
	resetLogs(t)
	var start, bucketSize int64 = 1200, 300
	end := start + bucketSize*11

	// 同一个请求：先在桶0 失败重试一次，然后在桶1 最终成功
	mkLogWithRID(t, LogTypeError, 1, 1, start+5, "retry-req")
	mkLogWithRID(t, LogTypeConsume, 1, 1, start+bucketSize+5, "retry-req")

	res, err := QueryErrorRate(0, nil, false, "1h", start, end, bucketSize)
	require.NoError(t, err)

	// 顶部汇总：1 次请求，最终成功
	assert.Equal(t, int64(0), res.ErrorCount, "请求最终成功不应计入 error_count")
	assert.Equal(t, int64(1), res.SuccessCount)
	assert.Equal(t, int64(1), res.Total)
	assert.Equal(t, 0.0, res.ErrorRate)

	// 趋势：请求只能在「首次时间」所在桶出现一次
	assert.Equal(t, int64(0), res.Trend[0].ErrorCount)
	assert.Equal(t, int64(1), res.Trend[0].SuccessCount, "应归入首次请求所在的桶0")
	assert.Equal(t, int64(0), res.Trend[1].ErrorCount, "成功日志所在桶不应另算一次")
	assert.Equal(t, int64(0), res.Trend[1].SuccessCount)

	// 顶部汇总 = 趋势求和（这是用户报告「对不上」的关键不变量）
	var sumErr, sumSucc int64
	for _, b := range res.Trend {
		sumErr += b.ErrorCount
		sumSucc += b.SuccessCount
	}
	assert.Equal(t, res.ErrorCount, sumErr)
	assert.Equal(t, res.SuccessCount, sumSucc)
}
