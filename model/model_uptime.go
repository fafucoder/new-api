package model

import (
	"sort"
	"strings"
	"time"
)

// ModelUptimeHistoryBucket is one cell in the per-model 24h history strip.
// Aggregation runs across every channel that serves the model.
type ModelUptimeHistoryBucket struct {
	Status     int   `json:"status"` // 1=any success, 0=all failure, -1=no data
	TsStart    int64 `json:"ts_start"`
	TsEnd      int64 `json:"ts_end"`
	SampleSize int   `json:"sample_size"`
}

// ModelChannelSnapshot is the admin-only per-channel current snapshot. Carries
// just the latest record metadata (no history) to keep payload small.
type ModelChannelSnapshot struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	Type         int    `json:"type"`
	Status       string `json:"status"` // normal | error | unknown
	StatusCode   int    `json:"status_code"`
	LatencyMs    int    `json:"latency_ms"`
	LastCheck    int64  `json:"last_check"`
	ErrorMessage string `json:"error,omitempty"`
}

// ModelUptimeAdminEntry is the per-model row returned to administrators.
type ModelUptimeAdminEntry struct {
	Model        string                     `json:"model"`
	Status       string                     `json:"status"`
	Uptime24h    *float64                   `json:"uptime_24h"`
	LastCheck    int64                      `json:"last_check"`
	ChannelCount int                        `json:"channel_count"`
	HealthyCount int                        `json:"healthy_count"`
	History      []ModelUptimeHistoryBucket `json:"history"`
	Channels     []ModelChannelSnapshot     `json:"channels"`
}

// ModelUptimePublicEntry is the desensitised per-model row returned to non-admin
// users. Deliberately omits per-channel identifiers, counts, errors and latency.
type ModelUptimePublicEntry struct {
	Model     string                     `json:"model"`
	Status    string                     `json:"status"`
	Uptime24h *float64                   `json:"uptime_24h"`
	History   []ModelUptimeHistoryBucket `json:"history"`
}

// modelStatusRank orders statuses so that failures sink to the top of the list.
// Lower rank means higher priority (rendered first). Same rank → alphabetical.
var modelStatusRank = map[string]int{
	"error":    0,
	"degraded": 1,
	"unknown":  2,
	"normal":   3,
}

func sortModelStatusKeys(a, b string, modelA, modelB string) bool {
	ra, ok := modelStatusRank[a]
	if !ok {
		ra = 4
	}
	rb, ok := modelStatusRank[b]
	if !ok {
		rb = 4
	}
	if ra != rb {
		return ra < rb
	}
	return modelA < modelB
}

// fetchModelChannelMap loads the (model, channel_id) pairs from the abilities
// table. Channels not present in allowedChannels (typically manually-disabled
// channels) are excluded. Pass nil groups for the admin view to skip the group
// filter; pass a non-empty slice to restrict to the user's groups.
func fetchModelChannelMap(groups []string, allowedChannels map[int]struct{}) (map[string]map[int]struct{}, error) {
	type abilityRow struct {
		Model     string
		ChannelId int
	}
	var rows []abilityRow

	query := DB.Table("abilities").
		Select("model, channel_id").
		Where("enabled = ?", commonTrueVal)

	if len(groups) > 0 {
		query = query.Where(commonGroupCol+" IN ?", groups)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string]map[int]struct{})
	for _, r := range rows {
		if r.Model == "" {
			continue
		}
		if allowedChannels != nil {
			if _, ok := allowedChannels[r.ChannelId]; !ok {
				continue
			}
		}
		if out[r.Model] == nil {
			out[r.Model] = make(map[int]struct{})
		}
		out[r.Model][r.ChannelId] = struct{}{}
	}
	return out, nil
}

// loadUptimeRecordsSince returns all channel_uptime_records in [since, now] for
// the given set of channel ids. Returns an empty slice (not nil) when no records
// match. Returns an empty slice (not error) for an empty channel id set.
func loadUptimeRecordsSince(channelIds []int, since int64) ([]ChannelUptimeRecord, error) {
	if len(channelIds) == 0 {
		return []ChannelUptimeRecord{}, nil
	}
	var records []ChannelUptimeRecord
	err := DB.Select("channel_id, status, status_code, response_time_ms, error_message, created_time").
		Where("channel_id IN ? AND created_time >= ?", channelIds, since).
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// buildLatestPerChannel reduces the record slice to the most recent record per
// channel_id. O(n) — avoids database-specific window functions for portability.
func buildLatestPerChannel(records []ChannelUptimeRecord) map[int]ChannelUptimeRecord {
	out := make(map[int]ChannelUptimeRecord, len(records))
	for _, r := range records {
		existing, ok := out[r.ChannelId]
		if !ok || r.CreatedTime > existing.CreatedTime {
			out[r.ChannelId] = r
		}
	}
	return out
}

// computeModelStatus combines per-channel latest records into an aggregate
// model status. successCount > 0 with no failures → normal; mixed → degraded;
// failures only → error; no records at all → unknown.
func computeModelStatus(channelIds map[int]struct{}, latestPerChannel map[int]ChannelUptimeRecord) (status string, healthy int) {
	successCount := 0
	failureCount := 0
	for cid := range channelIds {
		rec, ok := latestPerChannel[cid]
		if !ok {
			continue
		}
		if rec.Status == ChannelUptimeStatusSuccess {
			successCount++
		} else {
			failureCount++
		}
	}
	healthy = successCount
	switch {
	case successCount > 0 && failureCount == 0:
		status = "normal"
	case successCount > 0 && failureCount > 0:
		status = "degraded"
	case failureCount > 0:
		status = "error"
	default:
		status = "unknown"
	}
	return
}

// computeUptime24h returns the success rate over the 24h record set scoped to
// the model's channel ids. nil when there are no records in window.
func computeUptime24h(channelIds map[int]struct{}, records []ChannelUptimeRecord) *float64 {
	total := 0
	success := 0
	for _, r := range records {
		if _, ok := channelIds[r.ChannelId]; !ok {
			continue
		}
		total++
		if r.Status == ChannelUptimeStatusSuccess {
			success++
		}
	}
	if total == 0 {
		return nil
	}
	pct := float64(success) / float64(total) * 100.0
	return &pct
}

// computeLastCheck returns the maximum created_time across the model's channel
// latest records. Returns 0 when none of the channels has any record.
func computeLastCheck(channelIds map[int]struct{}, latestPerChannel map[int]ChannelUptimeRecord) int64 {
	var last int64
	for cid := range channelIds {
		if rec, ok := latestPerChannel[cid]; ok {
			if rec.CreatedTime > last {
				last = rec.CreatedTime
			}
		}
	}
	return last
}

// computeBuckets distributes records into 75 buckets across the last 24h.
// Each bucket: 1 if any channel had success, 0 if only failures, -1 if empty.
func computeBuckets(channelIds map[int]struct{}, records []ChannelUptimeRecord, now time.Time) []ModelUptimeHistoryBucket {
	const bucketCount = channelUptimeBucketCount
	bucketSpan := (24 * time.Hour) / time.Duration(bucketCount)
	spanSeconds := int64(bucketSpan / time.Second)
	if spanSeconds <= 0 {
		spanSeconds = 1
	}
	bucketStart := now.Add(-24 * time.Hour).Unix()

	buckets := make([]ModelUptimeHistoryBucket, bucketCount)
	for i := range buckets {
		buckets[i] = ModelUptimeHistoryBucket{
			Status:  -1,
			TsStart: bucketStart + int64(i)*spanSeconds,
			TsEnd:   bucketStart + int64(i+1)*spanSeconds,
		}
	}

	hasAny := make([]bool, bucketCount)
	hasSuccess := make([]bool, bucketCount)
	for _, r := range records {
		if _, ok := channelIds[r.ChannelId]; !ok {
			continue
		}
		idx := int((r.CreatedTime - bucketStart) / spanSeconds)
		if idx < 0 {
			idx = 0
		}
		if idx >= bucketCount {
			idx = bucketCount - 1
		}
		hasAny[idx] = true
		buckets[idx].SampleSize++
		if r.Status == ChannelUptimeStatusSuccess {
			hasSuccess[idx] = true
		}
	}
	for i := 0; i < bucketCount; i++ {
		switch {
		case hasSuccess[i]:
			buckets[i].Status = 1
		case hasAny[i]:
			buckets[i].Status = 0
		default:
			buckets[i].Status = -1
		}
	}
	return buckets
}

// buildChannelSnapshots produces the admin-only per-channel current snapshot
// array for a given model. The order is by channel id ascending.
func buildChannelSnapshots(
	channelIds map[int]struct{},
	latestPerChannel map[int]ChannelUptimeRecord,
	nameByID map[int]string,
	typeByID map[int]int,
) []ModelChannelSnapshot {
	out := make([]ModelChannelSnapshot, 0, len(channelIds))
	for cid := range channelIds {
		snap := ModelChannelSnapshot{
			Id:     cid,
			Name:   nameByID[cid],
			Type:   typeByID[cid],
			Status: "unknown",
		}
		if rec, ok := latestPerChannel[cid]; ok {
			snap.StatusCode = rec.StatusCode
			snap.LatencyMs = rec.ResponseTimeMs
			snap.LastCheck = rec.CreatedTime
			if rec.Status == ChannelUptimeStatusSuccess {
				snap.Status = "normal"
			} else {
				snap.Status = "error"
				snap.ErrorMessage = rec.ErrorMessage
			}
		}
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Id < out[j].Id
	})
	return out
}

// GetModelUptimeAdminViews returns one row per model (cross-group) with
// per-channel snapshot detail. Channels not present in allowedChannels are
// excluded entirely (typically manually-disabled channels).
//
// nameByID / typeByID provide channel display metadata for the snapshot field.
func GetModelUptimeAdminViews(
	allowedChannels map[int]struct{},
	nameByID map[int]string,
	typeByID map[int]int,
) ([]ModelUptimeAdminEntry, error) {
	modelToChannels, err := fetchModelChannelMap(nil, allowedChannels)
	if err != nil {
		return nil, err
	}

	allChannelIDs := collectChannelIDs(modelToChannels)
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour).Unix()

	records, err := loadUptimeRecordsSince(allChannelIDs, dayAgo)
	if err != nil {
		return nil, err
	}
	latestPerChannel := buildLatestPerChannel(records)

	entries := make([]ModelUptimeAdminEntry, 0, len(modelToChannels))
	for modelName, channelSet := range modelToChannels {
		status, healthy := computeModelStatus(channelSet, latestPerChannel)
		entries = append(entries, ModelUptimeAdminEntry{
			Model:        modelName,
			Status:       status,
			Uptime24h:    computeUptime24h(channelSet, records),
			LastCheck:    computeLastCheck(channelSet, latestPerChannel),
			ChannelCount: len(channelSet),
			HealthyCount: healthy,
			History:      computeBuckets(channelSet, records, now),
			Channels:     buildChannelSnapshots(channelSet, latestPerChannel, nameByID, typeByID),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return sortModelStatusKeys(entries[i].Status, entries[j].Status, entries[i].Model, entries[j].Model)
	})
	return entries, nil
}

// GetModelUptimePublicViews returns the desensitised per-model rows for a
// non-admin user. abilities are filtered to the user's groups; channels not
// present in allowedChannels are excluded.
func GetModelUptimePublicViews(
	allowedChannels map[int]struct{},
	groups []string,
) ([]ModelUptimePublicEntry, error) {
	if len(groups) == 0 {
		return []ModelUptimePublicEntry{}, nil
	}

	modelToChannels, err := fetchModelChannelMap(groups, allowedChannels)
	if err != nil {
		return nil, err
	}

	allChannelIDs := collectChannelIDs(modelToChannels)
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour).Unix()

	records, err := loadUptimeRecordsSince(allChannelIDs, dayAgo)
	if err != nil {
		return nil, err
	}
	latestPerChannel := buildLatestPerChannel(records)

	entries := make([]ModelUptimePublicEntry, 0, len(modelToChannels))
	for modelName, channelSet := range modelToChannels {
		status, _ := computeModelStatus(channelSet, latestPerChannel)
		entries = append(entries, ModelUptimePublicEntry{
			Model:     modelName,
			Status:    status,
			Uptime24h: computeUptime24h(channelSet, records),
			History:   computeBuckets(channelSet, records, now),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return sortModelStatusKeys(entries[i].Status, entries[j].Status, entries[i].Model, entries[j].Model)
	})
	return entries, nil
}

// collectChannelIDs flattens the (model → channel set) map into a deduped int
// slice suitable for an IN clause.
func collectChannelIDs(modelToChannels map[string]map[int]struct{}) []int {
	seen := make(map[int]struct{})
	for _, set := range modelToChannels {
		for cid := range set {
			seen[cid] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for cid := range seen {
		out = append(out, cid)
	}
	return out
}

// SplitUserGroups splits the comma-separated user.Group string into a deduped
// slice of non-empty group names. Returned in input order; empty input yields
// nil.
func SplitUserGroups(group string) []string {
	if group == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, 4)
	for _, raw := range strings.Split(group, ",") {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
