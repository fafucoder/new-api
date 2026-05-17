package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func truncateUptimeTable(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM channel_uptime_records").Error)
}

func TestRecordChannelUptime_TruncatesErrorMessage(t *testing.T) {
	truncateUptimeTable(t)

	longMsg := make([]byte, 1000)
	for i := range longMsg {
		longMsg[i] = 'x'
	}
	rec := &ChannelUptimeRecord{
		ChannelId:      1,
		ChannelType:    1,
		Status:         ChannelUptimeStatusFailure,
		ResponseTimeMs: 200,
		ErrorMessage:   string(longMsg),
	}
	require.NoError(t, RecordChannelUptime(rec))

	var loaded ChannelUptimeRecord
	require.NoError(t, DB.First(&loaded, rec.Id).Error)
	assert.Len(t, loaded.ErrorMessage, 500)
	assert.Greater(t, loaded.CreatedTime, int64(0))
}

func TestRecordChannelUptime_NilIsNoop(t *testing.T) {
	truncateUptimeTable(t)
	require.NoError(t, RecordChannelUptime(nil))

	var count int64
	require.NoError(t, DB.Model(&ChannelUptimeRecord{}).Count(&count).Error)
	assert.EqualValues(t, 0, count)
}

func TestCleanupExpiredUptimeRecords_OnlyDeletesOld(t *testing.T) {
	truncateUptimeTable(t)

	now := time.Now().Unix()
	old := &ChannelUptimeRecord{
		ChannelId: 1, ChannelType: 1, Status: ChannelUptimeStatusSuccess,
		CreatedTime: now - int64((8 * 24 * time.Hour).Seconds()),
	}
	fresh := &ChannelUptimeRecord{
		ChannelId: 1, ChannelType: 1, Status: ChannelUptimeStatusSuccess,
		CreatedTime: now - int64((1 * time.Hour).Seconds()),
	}
	require.NoError(t, DB.Create(old).Error)
	require.NoError(t, DB.Create(fresh).Error)

	require.NoError(t, CleanupExpiredUptimeRecords(7*24*time.Hour))

	var remaining []ChannelUptimeRecord
	require.NoError(t, DB.Find(&remaining).Error)
	require.Len(t, remaining, 1)
	assert.Equal(t, fresh.Id, remaining[0].Id)
}

func TestGetChannelUptimeAdminViews_NoRecords(t *testing.T) {
	truncateUptimeTable(t)

	views, err := GetChannelUptimeAdminViews([]int{42})
	require.NoError(t, err)
	require.Contains(t, views, 42)
	v := views[42]
	assert.Equal(t, "unknown", v.Status)
	assert.Empty(t, v.History)
	assert.Nil(t, v.Uptime24h)
}

func TestGetChannelUptimeAdminViews_HistoryAndUptime(t *testing.T) {
	truncateUptimeTable(t)

	now := time.Now().Unix()
	// 12 records: alternating success/failure within the last 24h.
	// Most recent (highest created_time) is success.
	for i := 0; i < 12; i++ {
		status := ChannelUptimeStatusSuccess
		if i%2 == 1 {
			status = ChannelUptimeStatusFailure
		}
		require.NoError(t, DB.Create(&ChannelUptimeRecord{
			ChannelId:      7,
			ChannelType:    14,
			Status:         status,
			StatusCode:     200,
			ResponseTimeMs: 100 + i,
			CreatedTime:    now - int64(i*60), // i minutes ago
		}).Error)
	}

	views, err := GetChannelUptimeAdminViews([]int{7})
	require.NoError(t, err)
	v := views[7]
	require.NotNil(t, v)

	assert.Equal(t, "normal", v.Status)
	assert.Equal(t, 14, v.ChannelType)
	assert.Equal(t, 100, v.LatencyMs) // most recent record (i=0)
	assert.Equal(t, 200, v.StatusCode)
	// All 12 records fit within the 75-record window; expect newest-first order.
	require.Len(t, v.History, 12)
	// Newest first: i=0 success, i=1 failure, ...
	assert.Equal(t, 1, v.History[0].Status)
	assert.Equal(t, 100, v.History[0].LatencyMs)
	assert.Equal(t, 0, v.History[1].Status)
	assert.Equal(t, 101, v.History[1].LatencyMs)
	// 12 records, 6 success → 50%
	require.NotNil(t, v.Uptime24h)
	assert.InDelta(t, 50.0, *v.Uptime24h, 0.01)
}

func TestGetChannelUptimeAdminViews_LatestIsFailureErrorPropagated(t *testing.T) {
	truncateUptimeTable(t)

	now := time.Now().Unix()
	require.NoError(t, DB.Create(&ChannelUptimeRecord{
		ChannelId: 1, ChannelType: 1, Status: ChannelUptimeStatusSuccess,
		CreatedTime: now - 120,
	}).Error)
	require.NoError(t, DB.Create(&ChannelUptimeRecord{
		ChannelId: 1, ChannelType: 1, Status: ChannelUptimeStatusFailure,
		ResponseTimeMs: 999, ErrorMessage: "boom",
		CreatedTime: now - 30,
	}).Error)

	views, err := GetChannelUptimeAdminViews([]int{1})
	require.NoError(t, err)
	v := views[1]
	assert.Equal(t, "error", v.Status)
	assert.Equal(t, "boom", v.ErrorMessage)
	assert.Equal(t, 999, v.LatencyMs)
}

func TestGetChannelUptimePublicViews_StatusAggregation(t *testing.T) {
	truncateUptimeTable(t)

	now := time.Now().Unix()
	// channel type 14: one success, one failure → degraded
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 101, ChannelType: 14, Status: ChannelUptimeStatusSuccess, CreatedTime: now - 60}).Error)
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 102, ChannelType: 14, Status: ChannelUptimeStatusFailure, CreatedTime: now - 60}).Error)
	// channel type 1: all success → normal
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 201, ChannelType: 1, Status: ChannelUptimeStatusSuccess, CreatedTime: now - 60}).Error)
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 202, ChannelType: 1, Status: ChannelUptimeStatusSuccess, CreatedTime: now - 60}).Error)
	// channel type 24: all failure → error
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 301, ChannelType: 24, Status: ChannelUptimeStatusFailure, CreatedTime: now - 60}).Error)
	// channel type 99: included in the map but has no records → unknown
	mapByType := map[int][]int{
		1:  {201, 202},
		14: {101, 102},
		24: {301},
		99: {999},
	}
	results, err := GetChannelUptimePublicViews(mapByType, 5)
	require.NoError(t, err)

	byType := map[int]ChannelUptimePublicView{}
	for _, r := range results {
		byType[r.ChannelType] = r
	}
	require.Contains(t, byType, 1)
	require.Contains(t, byType, 14)
	require.Contains(t, byType, 24)
	require.Contains(t, byType, 99)
	assert.Equal(t, "normal", byType[1].Status)
	assert.Equal(t, "degraded", byType[14].Status)
	assert.Equal(t, "error", byType[24].Status)
	assert.Equal(t, "unknown", byType[99].Status)
}

func TestGetChannelUptimePublicViews_HistoryBucketsThreeStates(t *testing.T) {
	truncateUptimeTable(t)

	now := time.Now().Unix()
	// 75 buckets, bucket span = (intervalMinutes+1) * 60s.
	const bucketCount = 75
	const intervalMinutes = 5
	bucketSpan := int64((intervalMinutes + 1) * 60)
	bucketStart := now - bucketCount*bucketSpan

	// Bucket 0: success → 1
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 5, Status: ChannelUptimeStatusSuccess, CreatedTime: bucketStart + 10}).Error)
	// Bucket 1: failure only → 0
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 5, Status: ChannelUptimeStatusFailure, CreatedTime: bucketStart + bucketSpan + 10}).Error)
	// Buckets 2..73: empty → -1
	// Bucket 74: failure + success → 1 (any success wins)
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 5, Status: ChannelUptimeStatusFailure, CreatedTime: bucketStart + (bucketCount-1)*bucketSpan + 60}).Error)
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 5, Status: ChannelUptimeStatusSuccess, CreatedTime: bucketStart + (bucketCount-1)*bucketSpan + 120}).Error)

	results, err := GetChannelUptimePublicViews(map[int][]int{5: {1}}, intervalMinutes)
	require.NoError(t, err)
	require.Len(t, results, 1)
	hist := results[0].History
	require.Len(t, hist, bucketCount)
	assert.Equal(t, 1, hist[0].Status)
	assert.Greater(t, hist[0].TsEnd, hist[0].TsStart)
	assert.Equal(t, int64(bucketSpan), hist[0].TsEnd-hist[0].TsStart, "bucket span must equal (interval+1)min")
	assert.Equal(t, 0, hist[1].Status)
	for i := 2; i <= bucketCount-2; i++ {
		assert.Equal(t, -1, hist[i].Status, "bucket %d should be unknown", i)
	}
	assert.Equal(t, 1, hist[bucketCount-1].Status)
	assert.Equal(t, 2, hist[bucketCount-1].SampleSize) // failure + success
}

// TestGetChannelUptimePublicViews_BucketsAnchoredToLatestProbe pins the
// rightmost bucket to the latest probe time so the strip does not drift
// with wall-clock now between refreshes. Three probes are inserted at
// known offsets behind the latest one; each must land in the bucket that
// matches its temporal distance, regardless of the test's wall-clock now.
func TestGetChannelUptimePublicViews_BucketsAnchoredToLatestProbe(t *testing.T) {
	truncateUptimeTable(t)

	const intervalMinutes = 5
	const bucketCount = 75
	bucketSpan := int64((intervalMinutes + 1) * 60)

	// Latest probe is 2 minutes in the past — comfortably below the 60s
	// buffer cap so historyEnd = latestProbe + 60 (no `now` cap).
	now := time.Now().Unix()
	latest := now - 120
	// Three probes spaced exactly one bucketSpan apart from `latest`,
	// going backwards. The rightmost bucket should contain `latest`,
	// the second-rightmost should contain `latest - bucketSpan`, etc.
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 7, Status: ChannelUptimeStatusSuccess, CreatedTime: latest}).Error)
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 7, Status: ChannelUptimeStatusSuccess, CreatedTime: latest - bucketSpan}).Error)
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 7, Status: ChannelUptimeStatusFailure, CreatedTime: latest - 2*bucketSpan}).Error)

	results, err := GetChannelUptimePublicViews(map[int][]int{7: {1}}, intervalMinutes)
	require.NoError(t, err)
	require.Len(t, results, 1)
	hist := results[0].History
	require.Len(t, hist, bucketCount)

	// historyEnd = latest + 60s; rightmost bucket = [historyEnd - span, historyEnd].
	historyEnd := latest + 60
	assert.Equal(t, historyEnd, hist[bucketCount-1].TsEnd, "rightmost bucket TsEnd must anchor to latest probe + 60s")
	assert.Equal(t, historyEnd-bucketSpan, hist[bucketCount-1].TsStart)

	// Rightmost bucket contains `latest`.
	assert.Equal(t, 1, hist[bucketCount-1].Status)
	assert.Equal(t, 1, hist[bucketCount-1].SampleSize)
	// Second-rightmost contains `latest - bucketSpan`.
	assert.Equal(t, 1, hist[bucketCount-2].Status)
	assert.Equal(t, 1, hist[bucketCount-2].SampleSize)
	// Third-rightmost contains `latest - 2*bucketSpan` (failure → status 0).
	assert.Equal(t, 0, hist[bucketCount-3].Status)
	assert.Equal(t, 1, hist[bucketCount-3].SampleSize)
	// Everything older is empty.
	for i := 0; i < bucketCount-3; i++ {
		assert.Equal(t, -1, hist[i].Status, "bucket %d should be unknown", i)
	}
}

// TestGetChannelUptimePublicViews_RecentProbeCapsAtNow guards the
// edge case where a probe fired in the last 60 seconds: anchor + 60
// would otherwise create a future-dated TsEnd, which we cap at now.
func TestGetChannelUptimePublicViews_RecentProbeCapsAtNow(t *testing.T) {
	truncateUptimeTable(t)

	const intervalMinutes = 5
	const bucketCount = 75

	// Probe 10 seconds ago. anchor + 60 = now + 50 → cap at now.
	beforeCall := time.Now().Unix()
	require.NoError(t, DB.Create(&ChannelUptimeRecord{ChannelId: 1, ChannelType: 7, Status: ChannelUptimeStatusSuccess, CreatedTime: beforeCall - 10}).Error)

	results, err := GetChannelUptimePublicViews(map[int][]int{7: {1}}, intervalMinutes)
	require.NoError(t, err)
	afterCall := time.Now().Unix()

	require.Len(t, results, 1)
	hist := results[0].History
	require.Len(t, hist, bucketCount)
	// TsEnd of the rightmost bucket must not exceed the wall-clock now
	// captured inside GetChannelUptimePublicViews. We can't observe that
	// exact value, but it's bounded by [beforeCall, afterCall].
	assert.LessOrEqual(t, hist[bucketCount-1].TsEnd, afterCall, "TsEnd must be capped at now")
	assert.GreaterOrEqual(t, hist[bucketCount-1].TsEnd, beforeCall, "TsEnd should be at or after the API call's now")
	// And the latest probe still lands in the rightmost bucket.
	assert.Equal(t, 1, hist[bucketCount-1].Status)
}
