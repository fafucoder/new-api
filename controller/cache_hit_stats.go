package controller

import (
	"math"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func parseTimeRange(c *gin.Context) (start, end int64) {
	if s := c.Query("start_time"); s != "" {
		start, _ = strconv.ParseInt(s, 10, 64)
	}
	if e := c.Query("end_time"); e != "" {
		end, _ = strconv.ParseInt(e, 10, 64)
	}
	if start == 0 && end == 0 {
		switch c.DefaultQuery("range", "today") {
		case "7d":
			start = time.Now().Unix() - 7*86400
		case "30d":
			start = time.Now().Unix() - 30*86400
		case "all":
			// no bounds
		default: // today
			now := time.Now()
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
		}
	}
	return
}

func hitRate(agg model.CacheHitStatsAggregate) interface{} {
	if agg.Total == 0 {
		return nil
	}
	r := math.Round(float64(agg.CacheRead)/float64(agg.Total)*10000) / 100
	return r
}

func GetMyCacheHitStats(c *gin.Context) {
	userId := c.GetInt("id")
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()

	today, err := model.AggregateCacheHitStatsByUser(userId, todayStart, 0)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "message": err.Error()})
		return
	}
	lifetime, err := model.AggregateCacheHitStatsByUser(userId, 0, 0)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "message": err.Error()})
		return
	}

	data := gin.H{
		"today":    aggToMap(today),
		"lifetime": aggToMap(lifetime),
	}

	start, end := parseTimeRange(c)
	if c.Query("start_time") != "" || c.Query("end_time") != "" {
		rangeAgg, err := model.AggregateCacheHitStatsByUser(userId, start, end)
		if err != nil {
			c.JSON(200, gin.H{"success": false, "message": err.Error()})
			return
		}
		data["range"] = aggToMap(rangeAgg)
	}

	c.JSON(200, gin.H{"success": true, "data": data})
}

func aggToMap(agg model.CacheHitStatsAggregate) gin.H {
	return gin.H{
		"cache_read":    agg.CacheRead,
		"input":         agg.Input,
		"cache_write":   agg.CacheWrite,
		"total":         agg.Total,
		"hit_rate":      hitRate(agg),
		"request_count": agg.RequestCount,
	}
}

func GetByChannelModelCacheHitStats(c *gin.Context) {
	start, end := parseTimeRange(c)
	rows, err := model.AggregateCacheHitStatsByChannelModel(start, end)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "message": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(rows))
	var sumAgg model.CacheHitStatsAggregate
	for _, r := range rows {
		ch, _ := model.CacheGetChannel(r.ChannelId)
		name, chType := "", 0
		if ch != nil {
			name, chType = ch.Name, ch.Type
		}
		sumAgg.CacheRead += r.CacheRead
		sumAgg.Input += r.Input
		sumAgg.CacheWrite += r.CacheWrite
		sumAgg.Total += r.Total
		sumAgg.RequestCount += r.RequestCount
		items = append(items, gin.H{
			"channel_id":    r.ChannelId,
			"channel_name":  name,
			"channel_type":  chType,
			"model_name":    r.ModelName,
			"cache_read":    r.CacheRead,
			"input":         r.Input,
			"cache_write":   r.CacheWrite,
			"total":         r.Total,
			"hit_rate":      hitRate(r.CacheHitStatsAggregate),
			"request_count": r.RequestCount,
		})
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"items": items, "summary": aggToMap(sumAgg)}})
}
