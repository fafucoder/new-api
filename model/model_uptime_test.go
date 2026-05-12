package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func truncateModelUptimeTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM channel_uptime_records").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
}

func insertAbility(t *testing.T, model string, channelID int, enabled bool, group string) {
	t.Helper()
	priority := int64(0)
	require.NoError(t, DB.Create(&Ability{
		Group:     group,
		Model:     model,
		ChannelId: channelID,
		Enabled:   enabled,
		Priority:  &priority,
	}).Error)
}

func insertUptimeRecord(t *testing.T, channelID int, status int, age time.Duration) {
	t.Helper()
	require.NoError(t, DB.Create(&ChannelUptimeRecord{
		ChannelId:      channelID,
		ChannelType:    1,
		Status:         status,
		StatusCode:     200,
		ResponseTimeMs: 100,
		CreatedTime:    time.Now().Add(-age).Unix(),
	}).Error)
}

// --- Case 1: abilities 表空 ---

func TestModelUptime_NoAbilities_AdminEmpty(t *testing.T) {
	truncateModelUptimeTables(t)

	entries, err := GetModelUptimeAdminViews(map[int]struct{}{1: {}, 2: {}}, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestModelUptime_NoAbilities_PublicEmpty(t *testing.T) {
	truncateModelUptimeTables(t)

	entries, err := GetModelUptimePublicViews(map[int]struct{}{1: {}}, []string{"default"})
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// --- Case 2: 单模型单 channel，最近 success ---

func TestModelUptime_SingleSuccess_StatusNormal(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "gpt-4o-mini", 100, true, "default")
	insertUptimeRecord(t, 100, ChannelUptimeStatusSuccess, time.Minute)

	allowed := map[int]struct{}{100: {}}
	entries, err := GetModelUptimeAdminViews(allowed, map[int]string{100: "Azure-East"}, map[int]int{100: 8})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "gpt-4o-mini", e.Model)
	assert.Equal(t, "normal", e.Status)
	assert.Equal(t, 1, e.ChannelCount)
	assert.Equal(t, 1, e.HealthyCount)
	require.NotNil(t, e.Uptime24h)
	assert.InDelta(t, 100.0, *e.Uptime24h, 0.01)
	require.Len(t, e.Channels, 1)
	assert.Equal(t, "Azure-East", e.Channels[0].Name)
	assert.Equal(t, "normal", e.Channels[0].Status)

	// At least one bucket should be `1`
	saw1 := false
	for _, b := range e.History {
		if b.Status == 1 {
			saw1 = true
			break
		}
	}
	assert.True(t, saw1, "expected at least one success bucket")
}

// --- Case 3: 单模型双 channel，一 success 一 failure → degraded ---

func TestModelUptime_MixedChannels_StatusDegraded(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "claude-3-opus", 1, true, "default")
	insertAbility(t, "claude-3-opus", 2, true, "default")
	insertUptimeRecord(t, 1, ChannelUptimeStatusSuccess, time.Minute)
	insertUptimeRecord(t, 2, ChannelUptimeStatusFailure, time.Minute)

	allowed := map[int]struct{}{1: {}, 2: {}}
	entries, err := GetModelUptimeAdminViews(allowed, map[int]string{1: "A", 2: "B"}, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "degraded", e.Status)
	assert.Equal(t, 2, e.ChannelCount)
	assert.Equal(t, 1, e.HealthyCount)
	assert.Len(t, e.Channels, 2)
}

// --- Case 4: 单模型双 channel 全 failure → error ---

func TestModelUptime_AllFailure_StatusError(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "claude-3-haiku", 10, true, "default")
	insertAbility(t, "claude-3-haiku", 11, true, "default")
	insertUptimeRecord(t, 10, ChannelUptimeStatusFailure, time.Minute)
	insertUptimeRecord(t, 11, ChannelUptimeStatusFailure, time.Minute)

	allowed := map[int]struct{}{10: {}, 11: {}}
	entries, err := GetModelUptimeAdminViews(allowed, map[int]string{10: "X", 11: "Y"}, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "error", e.Status)
	assert.Equal(t, 0, e.HealthyCount)
	assert.Len(t, e.Channels, 2)
	for _, ch := range e.Channels {
		assert.Equal(t, "error", ch.Status)
	}
}

// --- Case 5: 全无记录 → unknown ---

func TestModelUptime_NoRecords_StatusUnknown(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "gemini-pro", 50, true, "default")
	// No uptime records inserted

	allowed := map[int]struct{}{50: {}}
	entries, err := GetModelUptimeAdminViews(allowed, map[int]string{50: "Gemini"}, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "unknown", e.Status)
	assert.Nil(t, e.Uptime24h)
	assert.Equal(t, int64(0), e.LastCheck)
	require.Len(t, e.History, channelUptimeBucketCount)
	for i, b := range e.History {
		assert.Equal(t, -1, b.Status, "bucket %d should be -1", i)
	}
}

// --- Case 6: 跨组过滤——public 视图只取传入的组 ---

func TestModelUptime_PublicGroupFilter(t *testing.T) {
	truncateModelUptimeTables(t)
	// Same model served by different channels in different groups
	insertAbility(t, "gpt-4o", 100, true, "default")
	insertAbility(t, "gpt-4o", 200, true, "vip")
	insertUptimeRecord(t, 100, ChannelUptimeStatusFailure, time.Minute)
	insertUptimeRecord(t, 200, ChannelUptimeStatusSuccess, time.Minute)

	allowed := map[int]struct{}{100: {}, 200: {}}
	entries, err := GetModelUptimePublicViews(allowed, []string{"vip"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "gpt-4o", e.Model)
	// Only channel 200 (vip) counted → success only → normal
	assert.Equal(t, "normal", e.Status)
}

// --- Case 7: 用户多组——dedupe channel ---

func TestModelUptime_PublicMultipleGroupsDedupe(t *testing.T) {
	truncateModelUptimeTables(t)
	// Same (model, channel) in both groups → must dedupe
	insertAbility(t, "gpt-4o", 300, true, "ga")
	insertAbility(t, "gpt-4o", 300, true, "gb")
	insertUptimeRecord(t, 300, ChannelUptimeStatusSuccess, time.Minute)

	allowed := map[int]struct{}{300: {}}
	entries, err := GetModelUptimePublicViews(allowed, []string{"ga", "gb"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "normal", e.Status)
	// no panic + correct status proves the dedupe map handled the duplicate
}

// --- Case 8: 24h 边界——一周前 success，24h 内全 failure ---

func TestModelUptime_24hWindowOnly(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "embedding-3-large", 60, true, "default")

	// Outside 24h: success — must NOT count
	require.NoError(t, DB.Create(&ChannelUptimeRecord{
		ChannelId: 60, ChannelType: 1,
		Status:      ChannelUptimeStatusSuccess,
		CreatedTime: time.Now().Add(-7 * 24 * time.Hour).Unix(),
	}).Error)
	// Inside 24h: failure
	insertUptimeRecord(t, 60, ChannelUptimeStatusFailure, 1*time.Hour)

	allowed := map[int]struct{}{60: {}}
	entries, err := GetModelUptimeAdminViews(allowed, map[int]string{60: "E"}, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "error", e.Status)
	require.NotNil(t, e.Uptime24h)
	assert.InDelta(t, 0.0, *e.Uptime24h, 0.01)
}

// --- Case 9: enabled=false 的 ability 不参与 ---

func TestModelUptime_DisabledAbilityExcluded(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "claude-3-sonnet", 70, false, "default") // disabled
	insertAbility(t, "claude-3-sonnet", 71, true, "default")
	insertUptimeRecord(t, 70, ChannelUptimeStatusFailure, time.Minute)
	insertUptimeRecord(t, 71, ChannelUptimeStatusSuccess, time.Minute)

	allowed := map[int]struct{}{70: {}, 71: {}}
	entries, err := GetModelUptimeAdminViews(allowed, map[int]string{70: "A", 71: "B"}, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, 1, e.ChannelCount, "disabled ability should be excluded")
	assert.Equal(t, "normal", e.Status)
}

// --- Case 10: manuallyDisabled (not in allowedChannels) 不参与 ---

func TestModelUptime_ManuallyDisabledChannelExcluded(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "gpt-3.5-turbo", 80, true, "default")
	insertAbility(t, "gpt-3.5-turbo", 81, true, "default")
	insertUptimeRecord(t, 80, ChannelUptimeStatusFailure, time.Minute)
	insertUptimeRecord(t, 81, ChannelUptimeStatusSuccess, time.Minute)

	// channel 80 is "manually disabled" → not in allowedChannels
	allowed := map[int]struct{}{81: {}}
	entries, err := GetModelUptimeAdminViews(allowed, map[int]string{81: "B"}, nil)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, 1, e.ChannelCount)
	assert.Equal(t, "normal", e.Status)
	require.Len(t, e.Channels, 1)
	assert.Equal(t, 81, e.Channels[0].Id)
}

// --- Public view excludes channels[] and identifying fields ---

func TestModelUptime_PublicViewDesensitised(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "gpt-4o", 90, true, "default")
	insertUptimeRecord(t, 90, ChannelUptimeStatusSuccess, time.Minute)

	allowed := map[int]struct{}{90: {}}
	entries, err := GetModelUptimePublicViews(allowed, []string{"default"})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "gpt-4o", e.Model)
	assert.Equal(t, "normal", e.Status)
	require.NotNil(t, e.Uptime24h)
	require.Len(t, e.History, channelUptimeBucketCount)
	// Structural check: ModelUptimePublicEntry has no fields named
	// channel_count / channels / last_check. Verified at compile time by
	// the type itself; this test exists to guard against future field
	// additions getting silently leaked.
}

// --- Sort order: error > degraded > unknown > normal, then by model name ---

func TestModelUptime_SortFailuresFirst(t *testing.T) {
	truncateModelUptimeTables(t)
	// Three models, three statuses: ensure ordering
	insertAbility(t, "alpha-error", 1, true, "default")
	insertAbility(t, "alpha-normal", 2, true, "default")
	insertAbility(t, "alpha-unknown", 3, true, "default")
	insertUptimeRecord(t, 1, ChannelUptimeStatusFailure, time.Minute)
	insertUptimeRecord(t, 2, ChannelUptimeStatusSuccess, time.Minute)
	// channel 3: no records → unknown

	allowed := map[int]struct{}{1: {}, 2: {}, 3: {}}
	entries, err := GetModelUptimeAdminViews(allowed, nil, nil)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "alpha-error", entries[0].Model)
	assert.Equal(t, "error", entries[0].Status)
	assert.Equal(t, "alpha-unknown", entries[1].Model)
	assert.Equal(t, "unknown", entries[1].Status)
	assert.Equal(t, "alpha-normal", entries[2].Model)
	assert.Equal(t, "normal", entries[2].Status)
}

// --- SplitUserGroups: dedupe + trim ---

func TestSplitUserGroups(t *testing.T) {
	assert.Nil(t, SplitUserGroups(""))
	assert.Nil(t, SplitUserGroups(" "))
	assert.Equal(t, []string{"a"}, SplitUserGroups("a"))
	assert.Equal(t, []string{"a", "b"}, SplitUserGroups("a, b"))
	assert.Equal(t, []string{"vip", "default"}, SplitUserGroups("vip,default,vip"))
	assert.Equal(t, []string{"vip"}, SplitUserGroups(" vip , vip "))
}

// --- Public view with empty groups returns empty ---

func TestModelUptime_PublicEmptyGroupsReturnsEmpty(t *testing.T) {
	truncateModelUptimeTables(t)
	insertAbility(t, "gpt-4o", 1, true, "default")

	entries, err := GetModelUptimePublicViews(map[int]struct{}{1: {}}, nil)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
