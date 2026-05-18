package controller

import (
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetChannelUptimeStatus returns aggregated channel uptime information.
//
// Administrators receive a per-channel view (channel id, name, latency, error,
// recent history, 24h uptime %). Non-admin users receive a desensitised view
// aggregated by channel_type without channel names, ids, latency or error
// details. See docs/superpowers/specs/2026-05-13-channel-uptime-monitoring-design.md.
func GetChannelUptimeStatus(c *gin.Context) {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Filter out manually disabled channels — they are intentionally excluded
	// from monitoring (consistent with testAllChannels behaviour). Channels
	// whose setting marks DisableProbe=true are also excluded so they don't
	// pollute the status strip with stale "unknown" cells.
	active := make([]*model.Channel, 0, len(channels))
	for _, ch := range channels {
		if ch == nil || ch.Status == common.ChannelStatusManuallyDisabled {
			continue
		}
		if ch.GetSetting().DisableProbe {
			continue
		}
		active = append(active, ch)
	}

	now := common.GetTimestamp()
	role := c.GetInt("role")
	intervalMinutes := int(math.Round(operation_setting.GetMonitorSetting().AutoTestChannelMinutes))
	if intervalMinutes <= 0 {
		intervalMinutes = 5
	}

	if role >= common.RoleAdminUser {
		ids := make([]int, 0, len(active))
		nameByID := make(map[int]string, len(active))
		typeByID := make(map[int]int, len(active))
		for _, ch := range active {
			ids = append(ids, ch.Id)
			nameByID[ch.Id] = ch.Name
			typeByID[ch.Id] = ch.Type
		}

		views, err := model.GetChannelUptimeAdminViews(ids)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		services := make([]gin.H, 0, len(ids))
		for _, id := range ids {
			v := views[id]
			if v == nil {
				v = &model.ChannelUptimeAdminView{
					ChannelId: id,
					Status:    "unknown",
					History:   []model.ChannelUptimeHistoryEntry{},
				}
			}
			channelType := v.ChannelType
			if channelType == 0 {
				channelType = typeByID[id]
			}
			services = append(services, gin.H{
				"id":          id,
				"name":        nameByID[id],
				"type":        channelType,
				"status":      v.Status,
				"status_code": v.StatusCode,
				"latency_ms":  v.LatencyMs,
				"last_check":  v.LastCheck,
				"error":       v.ErrorMessage,
				"history":     v.History,
				"uptime_24h":  v.Uptime24h,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"view":             "admin",
				"services":         services,
				"interval_minutes": intervalMinutes,
				"updated_at":       now,
			},
		})
		return
	}

	// Public view: aggregate channels by type, omitting all identifying info.
	channelIdsByType := make(map[int][]int)
	for _, ch := range active {
		channelIdsByType[ch.Type] = append(channelIdsByType[ch.Type], ch.Id)
	}

	views, err := model.GetChannelUptimePublicViews(channelIdsByType, intervalMinutes)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"view":             "public",
			"services":         views,
			"interval_minutes": intervalMinutes,
			"updated_at":       now,
		},
	})
}
