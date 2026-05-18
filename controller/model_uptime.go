package controller

import (
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetModelUptimeStatus returns aggregated model uptime information.
//
// Administrators receive per-model rows with per-channel breakdown (id, name,
// type, status, status_code, latency, error). Non-admin users receive a
// desensitised per-model view filtered by their user.Group, with no channel
// identifiers, counts or errors. See
// docs/superpowers/specs/2026-05-13-model-uptime-monitoring-design.md.
func GetModelUptimeStatus(c *gin.Context) {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// allowedChannels excludes manually-disabled channels, mirroring
	// testAllChannels behaviour. Channels whose setting marks DisableProbe=true
	// are also excluded so they don't appear in per-model strip aggregations.
	// nameByID / typeByID populate the admin view's per-channel snapshot.
	allowed := make(map[int]struct{}, len(channels))
	nameByID := make(map[int]string, len(channels))
	typeByID := make(map[int]int, len(channels))
	for _, ch := range channels {
		if ch == nil || ch.Status == common.ChannelStatusManuallyDisabled {
			continue
		}
		if ch.GetSetting().DisableProbe {
			continue
		}
		allowed[ch.Id] = struct{}{}
		nameByID[ch.Id] = ch.Name
		typeByID[ch.Id] = ch.Type
	}

	now := common.GetTimestamp()
	role := c.GetInt("role")
	intervalMinutes := int(math.Round(operation_setting.GetMonitorSetting().AutoTestChannelMinutes))
	if intervalMinutes <= 0 {
		intervalMinutes = 5
	}

	if role >= common.RoleAdminUser {
		entries, err := model.GetModelUptimeAdminViews(allowed, nameByID, typeByID, intervalMinutes)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"view":             "admin",
				"models":           entries,
				"interval_minutes": intervalMinutes,
				"updated_at":       now,
			},
		})
		return
	}

	// Public view: filter abilities by the user's groups; strip identifying info.
	userID := c.GetInt("id")
	groupStr, err := model.GetUserGroup(userID, true)
	if err != nil {
		// Stale session or DB hiccup: fall back to empty groups → empty list,
		// rather than 500'ing the status page.
		groupStr = ""
	}
	groups := model.SplitUserGroups(groupStr)

	entries, err := model.GetModelUptimePublicViews(allowed, groups, intervalMinutes)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"view":             "public",
			"models":           entries,
			"interval_minutes": intervalMinutes,
			"updated_at":       now,
		},
	})
}
