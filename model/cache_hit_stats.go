package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

// CacheHitStats aggregates per (user, channel, model, hour) cache token usage.
// hit_rate = cache_read / (cache_read + input + cache_write); the rate itself
// is not stored — it is computed on read to avoid the "average of averages"
// trap when summing across rows.
type CacheHitStats struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_chs_user_hour,priority:1;uniqueIndex:uniq_chs_user_channel_model_hour,priority:1"`
	ChannelId        int    `json:"channel_id" gorm:"index;index:idx_chs_channel_hour,priority:1;uniqueIndex:uniq_chs_user_channel_model_hour,priority:2"`
	ModelName        string `json:"model_name" gorm:"size:128;default:'';uniqueIndex:uniq_chs_user_channel_model_hour,priority:3"`
	HourBucket       int64  `json:"hour_bucket" gorm:"bigint;index:idx_chs_user_hour,priority:2,sort:desc;index:idx_chs_channel_hour,priority:2,sort:desc;uniqueIndex:uniq_chs_user_channel_model_hour,priority:4"`
	CacheReadTokens  int64  `json:"cache_read_tokens" gorm:"default:0"`
	InputTokens      int64  `json:"input_tokens" gorm:"default:0"`
	CacheWriteTokens int64  `json:"cache_write_tokens" gorm:"default:0"`
	RequestCount     int64  `json:"request_count" gorm:"default:0"`
}

func (CacheHitStats) TableName() string {
	return "cache_hit_stats"
}

var (
	cacheHitStatsBuffer   = make(map[string]*CacheHitStats)
	cacheHitStatsBufferMu sync.Mutex
)

// LogCacheHitStats accumulates a single record into the in-memory buffer.
// Same key (user_id, channel_id, model_name, hour_bucket) merges into one row.
func LogCacheHitStats(userId, channelId int, modelName string, hour int64,
	cacheRead, input, cacheWrite int) {
	key := fmt.Sprintf("%d-%d-%s-%d", userId, channelId, modelName, hour)
	cacheHitStatsBufferMu.Lock()
	defer cacheHitStatsBufferMu.Unlock()
	if v, ok := cacheHitStatsBuffer[key]; ok {
		v.CacheReadTokens += int64(cacheRead)
		v.InputTokens += int64(input)
		v.CacheWriteTokens += int64(cacheWrite)
		v.RequestCount++
		return
	}
	cacheHitStatsBuffer[key] = &CacheHitStats{
		UserId:           userId,
		ChannelId:        channelId,
		ModelName:        modelName,
		HourBucket:       hour,
		CacheReadTokens:  int64(cacheRead),
		InputTokens:      int64(input),
		CacheWriteTokens: int64(cacheWrite),
		RequestCount:     1,
	}
}

// SaveCacheHitStatsCache flushes the buffer to DB. Each row is upserted with
// per-column additive accumulation via GORM's clause.OnConflict.Assignments,
// which translates to:
//   - PostgreSQL : INSERT ... ON CONFLICT (...) DO UPDATE SET col = col + EXCLUDED.col
//   - MySQL      : INSERT ... ON DUPLICATE KEY UPDATE col = col + VALUES(col)
//   - SQLite     : INSERT ... ON CONFLICT(...) DO UPDATE SET col = col + excluded.col
func SaveCacheHitStatsCache() {
	cacheHitStatsBufferMu.Lock()
	snapshot := cacheHitStatsBuffer
	cacheHitStatsBuffer = make(map[string]*CacheHitStats)
	cacheHitStatsBufferMu.Unlock()

	if len(snapshot) == 0 {
		return
	}

	for _, row := range snapshot {
		if err := DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"}, {Name: "channel_id"},
				{Name: "model_name"}, {Name: "hour_bucket"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"cache_read_tokens":  clause.Expr{SQL: "cache_hit_stats.cache_read_tokens + ?", Vars: []interface{}{row.CacheReadTokens}},
				"input_tokens":       clause.Expr{SQL: "cache_hit_stats.input_tokens + ?", Vars: []interface{}{row.InputTokens}},
				"cache_write_tokens": clause.Expr{SQL: "cache_hit_stats.cache_write_tokens + ?", Vars: []interface{}{row.CacheWriteTokens}},
				"request_count":      clause.Expr{SQL: "cache_hit_stats.request_count + ?", Vars: []interface{}{row.RequestCount}},
			}),
		}).Create(row).Error; err != nil {
			common.SysError(fmt.Sprintf("save cache_hit_stats failed: %s", err))
		}
	}
}

// UpdateCacheHitStats is a long-running goroutine that periodically flushes the
// in-memory buffer. Mirrors model.UpdateQuotaData.
func UpdateCacheHitStats() {
	for {
		time.Sleep(time.Duration(common.DataExportInterval) * time.Minute)
		SaveCacheHitStatsCache()
	}
}

// CacheHitStatsAggregate is the aggregated counters used both as table row
// summary and as item entries in API responses (without identity fields).
type CacheHitStatsAggregate struct {
	CacheRead    int64 `json:"cache_read"`
	Input        int64 `json:"input"`
	CacheWrite   int64 `json:"cache_write"`
	Total        int64 `json:"total"`
	RequestCount int64 `json:"request_count"`
}

// AggregateCacheHitStatsByUser sums all rows for the user within [start, end].
// Either bound being 0 means "open-ended" on that side. Returns zero-value
// aggregate (RequestCount == 0) when no rows match.
func AggregateCacheHitStatsByUser(userId int, start, end int64) (CacheHitStatsAggregate, error) {
	var out CacheHitStatsAggregate
	q := DB.Model(&CacheHitStats{}).
		Select("COALESCE(SUM(cache_read_tokens),0) AS cache_read, COALESCE(SUM(input_tokens),0) AS input, COALESCE(SUM(cache_write_tokens),0) AS cache_write, COALESCE(SUM(request_count),0) AS request_count").
		Where("user_id = ?", userId)
	if start > 0 {
		q = q.Where("hour_bucket >= ?", start)
	}
	if end > 0 {
		q = q.Where("hour_bucket <= ?", end)
	}
	if err := q.Scan(&out).Error; err != nil {
		return out, err
	}
	out.Total = out.CacheRead + out.Input + out.CacheWrite
	return out, nil
}

// CacheHitStatsByChannelModelRow is one row in the (channel, model) 2-D view.
type CacheHitStatsByChannelModelRow struct {
	ChannelId int    `json:"channel_id"`
	ModelName string `json:"model_name"`
	CacheHitStatsAggregate
}

// AggregateCacheHitStatsByChannelModel groups by (channel_id, model_name).
func AggregateCacheHitStatsByChannelModel(start, end int64) ([]CacheHitStatsByChannelModelRow, error) {
	q := DB.Model(&CacheHitStats{}).
		Select("channel_id, model_name, COALESCE(SUM(cache_read_tokens),0) AS cache_read, COALESCE(SUM(input_tokens),0) AS input, COALESCE(SUM(cache_write_tokens),0) AS cache_write, COALESCE(SUM(request_count),0) AS request_count").
		Group("channel_id, model_name")
	if start > 0 {
		q = q.Where("hour_bucket >= ?", start)
	}
	if end > 0 {
		q = q.Where("hour_bucket <= ?", end)
	}
	var rows []CacheHitStatsByChannelModelRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Total = rows[i].CacheRead + rows[i].Input + rows[i].CacheWrite
	}
	return rows, nil
}
