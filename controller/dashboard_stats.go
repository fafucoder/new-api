package controller

import (
	"math"
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

	cacheAgg, err := model.AggregateCacheHitStatsByUser(user.Id, start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var cacheHitRate any
	if cacheAgg.Total > 0 {
		cacheHitRate = math.Round(float64(cacheAgg.CacheRead)/float64(cacheAgg.Total)*10000) / 100
	}

	quotaRows, err := model.GetQuotaDataByUserId(user.Id, start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var totalQuota, totalTokens, totalCount int
	for _, row := range quotaRows {
		totalQuota += row.Quota
		totalTokens += row.TokenUsed
		totalCount += row.Count
	}

	common.ApiSuccess(c, gin.H{
		"user": gin.H{
			"id":            user.Id,
			"username":      user.Username,
			"quota":         user.Quota,
			"used_quota":    user.UsedQuota,
			"request_count": user.RequestCount,
		},
		"cache_hit_stats": gin.H{
			"cache_read":    cacheAgg.CacheRead,
			"input":         cacheAgg.Input,
			"cache_write":   cacheAgg.CacheWrite,
			"total":         cacheAgg.Total,
			"hit_rate":      cacheHitRate,
			"request_count": cacheAgg.RequestCount,
		},
		"quota_stats": gin.H{
			"total_quota":  totalQuota,
			"total_tokens": totalTokens,
			"total_count":  totalCount,
		},
	})
}
