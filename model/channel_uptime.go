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
)

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
func GetChannelUptimePublicViews(channelIdsByType map[int][]int) ([]ChannelUptimePublicView, error) {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour).Unix()
	bucketSpan := (24 * time.Hour) / time.Duration(channelUptimeBucketCount)
	bucketStart := now.Add(-24 * time.Hour).Unix()

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
		spanSeconds := int64(bucketSpan / time.Second)
		if spanSeconds <= 0 {
			spanSeconds = 1
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

		// 10-bucket history across the last 24h.
		var bucketRecords []ChannelUptimeRecord
		if err := DB.Select("status", "created_time").
			Where("channel_id IN ? AND created_time >= ?", ids, dayAgo).
			Find(&bucketRecords).Error; err != nil {
			return nil, err
		}
		bucketHasAny := make([]bool, channelUptimeBucketCount)
		bucketHasSuccess := make([]bool, channelUptimeBucketCount)
		bucketSampleSize := make([]int, channelUptimeBucketCount)
		for _, r := range bucketRecords {
			idx := int((r.CreatedTime - bucketStart) / spanSeconds)
			if idx < 0 {
				idx = 0
			}
			if idx >= channelUptimeBucketCount {
				idx = channelUptimeBucketCount - 1
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
