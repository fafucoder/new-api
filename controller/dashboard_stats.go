package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetUserDashboardStats(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		common.ApiErrorMsg(c, "username 不能为空")
		return
	}

	user, err := model.GetUserByUsername(username)
	if err != nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	now := time.Now()
	var start, end int64

	if st := c.Query("start_timestamp"); st != "" {
		start, _ = strconv.ParseInt(st, 10, 64)
	}
	if et := c.Query("end_timestamp"); et != "" {
		end, _ = strconv.ParseInt(et, 10, 64)
	}

	if start == 0 || end == 0 {
		end = now.Unix()
		switch c.DefaultQuery("range", "7d") {
		case "1d":
			start = now.Unix() - 86400
		case "30d":
			start = now.Unix() - 30*86400
		default:
			start = now.Unix() - 7*86400
		}
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	todayCache, err := model.AggregateCacheHitStatsByUser(user.Id, todayStart, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	lifetimeCache, err := model.AggregateCacheHitStatsByUser(user.Id, 0, 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rangeCache, err := model.AggregateCacheHitStatsByUser(user.Id, start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 真实消耗数据基于 logs 表统计
	stat, err := model.SumUsedQuota(model.LogTypeConsume, start, end, "", username, "", 0, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rangeUsedQuota := stat.Quota
	rangeRequestCount := model.SumUsedRequestCount(start, end, username)

	// 按模型分组的真实消耗数据
	quotaDataByModel, err := model.SumUsedQuotaGroupByModel(start, end, username)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"user": gin.H{
			"id":            user.Id,
			"username":      user.Username,
			"quota":         user.Quota,
			"used_quota":    rangeUsedQuota,
			"request_count": rangeRequestCount,
		},
		"cache_hit_stats": gin.H{
			"today":    aggToMap(todayCache),
			"lifetime": aggToMap(lifetimeCache),
			"range":    aggToMap(rangeCache),
		},
		"quota_data": quotaDataByModel,
	})
}
