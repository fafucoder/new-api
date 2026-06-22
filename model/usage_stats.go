package model

import (
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
)

// ErrorRateBucket 单个时间桶的错误率数据
type ErrorRateBucket struct {
	Time         int64   `json:"time"`
	ErrorCount   int64   `json:"error_count"`
	SuccessCount int64   `json:"success_count"`
	ErrorRate    float64 `json:"error_rate"`
}

// ErrorRateResult 错误率统计结果
type ErrorRateResult struct {
	Window       string            `json:"window"`
	ErrorCount   int64             `json:"error_count"`
	SuccessCount int64             `json:"success_count"`
	Total        int64             `json:"total"`
	ErrorRate    float64           `json:"error_rate"`
	Trend        []ErrorRateBucket `json:"trend"`
}

// errorRateRow 聚合查询扫描结果
type errorRateRow struct {
	Bucket       int64
	ErrorCount   int64
	SuccessCount int64
}

// calcErrorRate 错误率(百分比, 2位小数)，total=0 时返回 0
func calcErrorRate(errorCount, successCount int64) float64 {
	total := errorCount + successCount
	if total == 0 {
		return 0
	}
	return math.Round(float64(errorCount)/float64(total)*10000) / 100
}

// QueryErrorRate 统计 [start,end] 内按 bucketSize 分桶的错误率。
// userId>0 限定该用户；applyChannelFilter=true 时按 channelIDs 过滤(空列表=命中0行)。
// 按 request_id 去重：同一 request_id 的多次重试只计算最终结果，
// 并且把该请求归入 MIN(created_at) 所在的桶，避免跨桶重试被拆成两个请求
// （这是「顶部错误率与下方 error_count/success_count/total 对不上」的根因）。
func QueryErrorRate(userId int, channelIDs []int, applyChannelFilter bool, window string, start, end, bucketSize int64) (ErrorRateResult, error) {
	result := ErrorRateResult{Window: window, Trend: []ErrorRateBucket{}}
	if bucketSize <= 0 {
		bucketSize = 1
	}

	// 分桶表达式 first_ts / bucketSize（整除）。
	// MySQL 的 / 返回小数需 FLOOR；SQLite/PostgreSQL 整数 / 整数即整除。
	// 注意：必须基于「请求首次时间」分桶，不能直接用 created_at，
	// 否则一个请求的重试错误日志和最终成功日志若跨桶，会被同时计入两个桶的
	// error_count / success_count，造成与顶部聚合数对不上。
	var bucketExpr string
	if common.UsingMySQL {
		bucketExpr = "FLOOR(first_ts / ?)"
	} else {
		bucketExpr = "(first_ts / ?)"
	}

	// 子查询：按 request_id 聚合到「请求维度」。
	// has_success = 1 表示该请求最终成功（至少一条 LogTypeConsume）。
	// first_ts = 该请求最早一条日志的时间，用于稳定分桶。
	subQuery := LOG_DB.Table("logs").
		Select("request_id, "+
			"MAX(CASE WHEN type = ? THEN 1 ELSE 0 END) AS has_success, "+
			"MIN(created_at) AS first_ts",
			LogTypeConsume).
		Where("created_at >= ? AND created_at <= ?", start, end).
		Where("type IN (?)", []int{LogTypeConsume, LogTypeError}).
		Where("request_id != ?", "") // 只统计有 request_id 的日志

	if userId > 0 {
		subQuery = subQuery.Where("user_id = ?", userId)
	}
	if applyChannelFilter {
		subQuery = subQuery.Where("channel_id IN (?)", channelIDs)
	}

	subQuery = subQuery.Group("request_id")

	// 外层查询：按「请求首次时间所在桶」聚合，统计成功和失败的请求数
	query := LOG_DB.Table("(?) AS req", subQuery).
		Select(bucketExpr+" AS bucket, "+
			"SUM(CASE WHEN has_success = 0 THEN 1 ELSE 0 END) AS error_count, "+
			"SUM(CASE WHEN has_success = 1 THEN 1 ELSE 0 END) AS success_count",
			bucketSize).
		Group("bucket")

	var rows []errorRateRow
	if err := query.Scan(&rows).Error; err != nil {
		common.SysError("failed to query error rate: " + err.Error())
		return result, err
	}

	// 调试日志
	common.SysLog(fmt.Sprintf("QueryErrorRate debug: start=%d, end=%d, bucketSize=%d, rowCount=%d", start, end, bucketSize, len(rows)))
	if len(rows) > 0 {
		common.SysLog(fmt.Sprintf("QueryErrorRate first row: bucket=%d, error=%d, success=%d", rows[0].Bucket, rows[0].ErrorCount, rows[0].SuccessCount))
	}

	// 桶起始时间 -> 行
	// bucket 字段是 created_at / bucketSize 的结果，需要乘回来得到时间戳
	rowMap := make(map[int64]errorRateRow, len(rows))
	for _, r := range rows {
		bucketTime := r.Bucket * bucketSize
		rowMap[bucketTime] = r
	}

	// 桶序列对齐到 bucketSize 网格，保证与 rowMap key 匹配
	alignedStart := (start / bucketSize) * bucketSize
	var totalErr, totalSucc int64
	for tt := alignedStart; tt <= end; tt += bucketSize {
		r := rowMap[tt]
		totalErr += r.ErrorCount
		totalSucc += r.SuccessCount
		result.Trend = append(result.Trend, ErrorRateBucket{
			Time:         tt,
			ErrorCount:   r.ErrorCount,
			SuccessCount: r.SuccessCount,
			ErrorRate:    calcErrorRate(r.ErrorCount, r.SuccessCount),
		})
	}

	result.ErrorCount = totalErr
	result.SuccessCount = totalSucc
	result.Total = totalErr + totalSucc
	result.ErrorRate = calcErrorRate(totalErr, totalSucc)
	return result, nil
}
