package model

import (
	"errors"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ChannelUptimeStatusSuccess = 1
	ChannelUptimeStatusFailure = 2

	channelUptimeErrorMessageMax = 500
	channelUptimeHistoryLimit    = 75
	channelUptimeBucketCount     = 75
	channelUptimeDefaultInterval = 5
)

// channelUptimeBucketSpan returns the time covered by a single cell on the
// history strip. The +1 minute buffer aligns probe schedule with bucket
// boundaries so each scheduled probe lands in exactly one bucket — preventing
// the "暂无数据" gaps that appear when buckets are narrower than the probe
// interval.
func channelUptimeBucketSpan(intervalMinutes int) time.Duration {
	if intervalMinutes <= 0 {
		intervalMinutes = channelUptimeDefaultInterval
	}
	return time.Duration(intervalMinutes+1) * time.Minute
}

// channelUptimeHistoryWindow is the total span of the bucket strip
// (channelUptimeBucketCount cells wide). Window stretches with the probe
// interval — 5min → 7.5h, 30min → ~38h, 60min → ~76h.
func channelUptimeHistoryWindow(intervalMinutes int) time.Duration {
	return time.Duration(channelUptimeBucketCount) * channelUptimeBucketSpan(intervalMinutes)
}

// channelUptimeLatestRecordTime returns the maximum created_time across the
// given channel ids, or 0 if no records exist. Used as the anchor for the
// history strip so bucket boundaries align with the actual probe schedule
// instead of drifting with the API call's wall-clock now.
//
// COALESCE keeps the result cross-database (SQLite/MySQL/PostgreSQL); the
// MAX aggregation uses the channel_id + created_time composite index.
func channelUptimeLatestRecordTime(channelIds []int) (int64, error) {
	if len(channelIds) == 0 {
		return 0, nil
	}
	var result struct {
		MaxTs int64
	}
	err := DB.Model(&ChannelUptimeRecord{}).
		Where("channel_id IN ?", channelIds).
		Select("COALESCE(MAX(created_time), 0) AS max_ts").
		Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.MaxTs, nil
}

// channelUptimeHistoryEnd returns the unix timestamp where the rightmost
// history bucket ends. Anchored to anchor + 60s (a 1-minute buffer past the
// latest probe so the probe sits comfortably inside the rightmost bucket
// instead of on its edge), capped at now to avoid future-dated buckets.
// Falls back to now when no probes exist.
func channelUptimeHistoryEnd(anchor int64, now time.Time) int64 {
	nowUnix := now.Unix()
	if anchor <= 0 {
		return nowUnix
	}
	end := anchor + 60
	if end > nowUnix {
		end = nowUnix
	}
	return end
}

type ChannelUptimeRecord struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ChannelId      int    `json:"channel_id" gorm:"index:idx_channel_uptime_channel_created,priority:1"`
	ChannelType    int    `json:"channel_type" gorm:"index"`
	Status         int    `json:"status"`
	StatusCode     int    `json:"status_code"`
	ResponseTimeMs int    `json:"response_time_ms"`
	ErrorMessage   string `json:"error_message" gorm:"type:varchar(512)"`
	CreatedTime    int64  `json:"created_time" gorm:"bigint;index;index:idx_channel_uptime_channel_created,priority:2,sort:desc"`
}

func (ChannelUptimeRecord) TableName() string {
	return "channel_uptime_records"
}

// RecordChannelUptime persists a single channel test outcome. The function is
// expected to be called from a background goroutine; failures are surfaced via
// the returned error but the caller typically only logs them.
func RecordChannelUptime(record *ChannelUptimeRecord) error {
	if record == nil {
		return nil
	}
	if len(record.ErrorMessage) > channelUptimeErrorMessageMax {
		record.ErrorMessage = record.ErrorMessage[:channelUptimeErrorMessageMax]
	}
	if record.CreatedTime == 0 {
		record.CreatedTime = common.GetTimestamp()
	}
	return DB.Create(record).Error
}

// CleanupExpiredUptimeRecords removes records older than the given retention
// window. Uses a plain DELETE that is valid across SQLite, MySQL and Postgres.
func CleanupExpiredUptimeRecords(retention time.Duration) error {
	cutoff := time.Now().Add(-retention).Unix()
	return DB.Where("created_time < ?", cutoff).Delete(&ChannelUptimeRecord{}).Error
}

// ChannelUptimeHistoryEntry is one cell in the admin history strip. Carries
// enough metadata to support a click-to-inspect tooltip on the frontend.
type ChannelUptimeHistoryEntry struct {
	Status       int    `json:"status"` // 1=success, 0=failure
	StatusCode   int    `json:"status_code"`
	LatencyMs    int    `json:"latency_ms"`
	Ts           int64  `json:"ts"`
	ErrorMessage string `json:"error,omitempty"`
}

// ChannelUptimeBucketEntry is one cell in the public history strip.
// Aggregated across all channels of a type for a 24h-window time bucket.
// Deliberately omits any per-channel detail (names, errors, latency).
type ChannelUptimeBucketEntry struct {
	Status     int   `json:"status"` // 1=any success, 0=all failure, -1=no data
	TsStart    int64 `json:"ts_start"`
	TsEnd      int64 `json:"ts_end"`
	SampleSize int   `json:"sample_size"`
}

// ChannelUptimeAdminView aggregates the recent uptime history for a single
// channel. Designed to be returned to administrators.
type ChannelUptimeAdminView struct {
	ChannelId    int                         `json:"id"`
	ChannelType  int                         `json:"type"`
	Status       string                      `json:"status"`
	StatusCode   int                         `json:"status_code"`
	LatencyMs    int                         `json:"latency_ms"`
	LastCheck    int64                       `json:"last_check"`
	ErrorMessage string                      `json:"error"`
	History      []ChannelUptimeHistoryEntry `json:"history"`
	Uptime24h    *float64                    `json:"uptime_24h"`
}

// GetChannelUptimeAdminViews returns one aggregated entry per channel id
// supplied. Channels without records are returned with status="unknown".
func GetChannelUptimeAdminViews(channelIds []int) (map[int]*ChannelUptimeAdminView, error) {
	views := make(map[int]*ChannelUptimeAdminView, len(channelIds))
	if len(channelIds) == 0 {
		return views, nil
	}

	dayAgo := time.Now().Add(-24 * time.Hour).Unix()

	for _, id := range channelIds {
		view := &ChannelUptimeAdminView{
			ChannelId: id,
			Status:    "unknown",
			History:   []ChannelUptimeHistoryEntry{},
		}

		var recent []ChannelUptimeRecord
		if err := DB.Where("channel_id = ?", id).
			Order("created_time DESC").
			Limit(channelUptimeHistoryLimit).
			Find(&recent).Error; err != nil {
			return nil, err
		}

		if len(recent) > 0 {
			latest := recent[0]
			view.ChannelType = latest.ChannelType
			view.LatencyMs = latest.ResponseTimeMs
			view.LastCheck = latest.CreatedTime
			view.StatusCode = latest.StatusCode
			if latest.Status == ChannelUptimeStatusSuccess {
				view.Status = "normal"
			} else {
				view.Status = "error"
				view.ErrorMessage = latest.ErrorMessage
			}
			history := make([]ChannelUptimeHistoryEntry, len(recent))
			for i, r := range recent {
				entry := ChannelUptimeHistoryEntry{
					StatusCode: r.StatusCode,
					LatencyMs:  r.ResponseTimeMs,
					Ts:         r.CreatedTime,
				}
				if r.Status == ChannelUptimeStatusSuccess {
					entry.Status = 1
				} else {
					entry.Status = 0
					entry.ErrorMessage = r.ErrorMessage
				}
				history[i] = entry
			}
			view.History = history
		}

		var stats struct {
			Total   int64
			Success int64
		}
		if err := DB.Model(&ChannelUptimeRecord{}).
			Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success", ChannelUptimeStatusSuccess).
			Where("channel_id = ? AND created_time >= ?", id, dayAgo).
			Scan(&stats).Error; err != nil {
			return nil, err
		}
		if stats.Total > 0 {
			pct := float64(stats.Success) / float64(stats.Total) * 100.0
			view.Uptime24h = &pct
		}

		views[id] = view
	}

	return views, nil
}

// ChannelUptimePublicView is the desensitised, per-type aggregation shown to
// non-admin users. It deliberately omits channel ids, names, latency and
// error messages.
type ChannelUptimePublicView struct {
	ChannelType int                        `json:"type"`
	Status      string                     `json:"status"`
	Uptime24h   *float64                   `json:"uptime_24h"`
	History     []ChannelUptimeBucketEntry `json:"history"`
}

// GetChannelUptimePublicViews returns one aggregated entry per channel_type
// present in channelIds. The aggregation rules are documented in
// docs/superpowers/specs/2026-05-13-channel-uptime-monitoring-design.md.
//
// intervalMinutes is the configured probe interval; bucket span follows
// channelUptimeBucketSpan(intervalMinutes) so each probe maps to one bucket.
// The rightmost bucket is anchored to the most recent probe across all
// channels in scope, eliminating drift between the strip's bucket grid and
// the probe schedule.
func GetChannelUptimePublicViews(channelIdsByType map[int][]int, intervalMinutes int) ([]ChannelUptimePublicView, error) {
	now := time.Now()
	bucketSpan := channelUptimeBucketSpan(intervalMinutes)
	historyWindow := channelUptimeHistoryWindow(intervalMinutes)

	// Uptime% retains its 24h semantics regardless of strip window.
	dayAgo := now.Add(-24 * time.Hour).Unix()

	// Anchor the bucket grid to the latest probe across all channels in
	// scope so the strip doesn't drift with each API call's `now`. One
	// anchor for all types keeps the visual alignment consistent across
	// rows on the same page.
	allIds := make([]int, 0)
	for _, ids := range channelIdsByType {
		allIds = append(allIds, ids...)
	}
	anchor, err := channelUptimeLatestRecordTime(allIds)
	if err != nil {
		return nil, err
	}
	historyEnd := channelUptimeHistoryEnd(anchor, now)

	spanSeconds := int64(bucketSpan / time.Second)
	if spanSeconds <= 0 {
		spanSeconds = 1
	}
	bucketStart := historyEnd - int64(historyWindow/time.Second)

	results := make([]ChannelUptimePublicView, 0, len(channelIdsByType))

	for channelType, ids := range channelIdsByType {
		if len(ids) == 0 {
			continue
		}

		view := ChannelUptimePublicView{
			ChannelType: channelType,
			Status:      "unknown",
			History:     make([]ChannelUptimeBucketEntry, channelUptimeBucketCount),
		}
		for i := range view.History {
			view.History[i] = ChannelUptimeBucketEntry{
				Status:  -1,
				TsStart: bucketStart + int64(i)*spanSeconds,
				TsEnd:   bucketStart + int64(i+1)*spanSeconds,
			}
		}

		// Latest record per channel determines aggregate current status.
		successCount := 0
		failureCount := 0
		for _, id := range ids {
			var latest ChannelUptimeRecord
			err := DB.Where("channel_id = ?", id).Order("created_time DESC").Limit(1).First(&latest).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return nil, err
			}
			if latest.Status == ChannelUptimeStatusSuccess {
				successCount++
			} else {
				failureCount++
			}
		}
		switch {
		case successCount > 0 && failureCount == 0:
			view.Status = "normal"
		case successCount > 0 && failureCount > 0:
			view.Status = "degraded"
		case successCount == 0 && failureCount > 0:
			view.Status = "error"
		default:
			view.Status = "unknown"
		}

		// 24h overall uptime % across all channels of this type.
		var stats struct {
			Total   int64
			Success int64
		}
		if err := DB.Model(&ChannelUptimeRecord{}).
			Select("COUNT(*) AS total, COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success", ChannelUptimeStatusSuccess).
			Where("channel_id IN ? AND created_time >= ?", ids, dayAgo).
			Scan(&stats).Error; err != nil {
			return nil, err
		}
		if stats.Total > 0 {
			pct := float64(stats.Success) / float64(stats.Total) * 100.0
			view.Uptime24h = &pct
		}

		// Bucket history across the strip window (75 cells of bucketSpan).
		var bucketRecords []ChannelUptimeRecord
		if err := DB.Select("status", "created_time").
			Where("channel_id IN ? AND created_time >= ?", ids, bucketStart).
			Find(&bucketRecords).Error; err != nil {
			return nil, err
		}
		bucketHasAny := make([]bool, channelUptimeBucketCount)
		bucketHasSuccess := make([]bool, channelUptimeBucketCount)
		bucketSampleSize := make([]int, channelUptimeBucketCount)
		for _, r := range bucketRecords {
			if r.CreatedTime < bucketStart {
				continue
			}
			idx := int((r.CreatedTime - bucketStart) / spanSeconds)
			if idx < 0 || idx >= channelUptimeBucketCount {
				continue
			}
			bucketHasAny[idx] = true
			bucketSampleSize[idx]++
			if r.Status == ChannelUptimeStatusSuccess {
				bucketHasSuccess[idx] = true
			}
		}
		for i := 0; i < channelUptimeBucketCount; i++ {
			view.History[i].SampleSize = bucketSampleSize[i]
			switch {
			case bucketHasSuccess[i]:
				view.History[i].Status = 1
			case bucketHasAny[i]:
				view.History[i].Status = 0
			default:
				view.History[i].Status = -1
			}
		}

		results = append(results, view)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ChannelType < results[j].ChannelType
	})
	return results, nil
}
