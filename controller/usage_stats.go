package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// windowConfig 窗口配置（单位：秒）
type windowConfig struct {
	bucketSize int64
	bucketNum  int64
}

var errorRateWindows = map[string]windowConfig{
	"5m":  {30, 10},
	"30m": {120, 15},
	"1h":  {300, 12},
	"2h":  {600, 12},
	"1d":  {3600, 24},
}

// resolveWindow 解析窗口参数，非法值回退 1h
func resolveWindow(w string) (string, windowConfig) {
	if cfg, ok := errorRateWindows[w]; ok {
		return w, cfg
	}
	return "1h", errorRateWindows["1h"]
}

// computeRange 按 bucketSize 网格对齐计算 [start,end]，使趋势桶数恒等于 bucketNum
func computeRange(cfg windowConfig) (start, end int64) {
	now := time.Now().Unix()
	lastBucket := (now / cfg.bucketSize) * cfg.bucketSize
	start = lastBucket - (cfg.bucketNum-1)*cfg.bucketSize
	end = now
	return
}

// pickBucketSize 根据时间跨度(秒)选择合适的分桶大小，使趋势点数适中
func pickBucketSize(span int64) int64 {
	switch {
	case span <= 600: // <=10min
		return 30
	case span <= 3600: // <=1h
		return 300
	case span <= 21600: // <=6h
		return 1800
	case span <= 86400: // <=1d
		return 3600
	case span <= 604800: // <=7d
		return 21600
	default:
		return 86400
	}
}

// resolveTimeRange 解析时间范围：优先自定义 start_time/end_time(unix秒)，否则用 window 预设
func resolveTimeRange(c *gin.Context) (window string, start, end, bucketSize int64) {
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")
	if startStr != "" && endStr != "" {
		s, _ := strconv.ParseInt(startStr, 10, 64)
		e, _ := strconv.ParseInt(endStr, 10, 64)
		if s > 0 && e > s {
			return "custom", s, e, pickBucketSize(e - s)
		}
	}
	w, cfg := resolveWindow(c.Query("window"))
	s, e := computeRange(cfg)
	return w, s, e, cfg.bucketSize
}

// resolveChannelFilter 解析管理员的渠道/标签筛选。
// 返回 (channelIDs, applyFilter)：applyFilter=false 表示不限定渠道。
func resolveChannelFilter(c *gin.Context) ([]int, bool) {
	cidStr := c.Query("channel_id")
	tag := c.Query("tag")
	if cidStr == "" && tag == "" {
		return nil, false
	}
	cid, _ := strconv.Atoi(cidStr)

	if tag != "" {
		channels, err := model.GetChannelsByTag(tag, false, false)
		tagIDs := make([]int, 0)
		if err == nil {
			for _, ch := range channels {
				tagIDs = append(tagIDs, ch.Id)
			}
		}
		if cid > 0 {
			// 交集：channel_id 必须在 tag 渠道集合内
			inter := make([]int, 0)
			for _, id := range tagIDs {
				if id == cid {
					inter = append(inter, id)
				}
			}
			return inter, true
		}
		return tagIDs, true
	}
	if cid > 0 {
		return []int{cid}, true
	}
	// channel_id 提供但非法 → 命中空集
	return []int{}, true
}

// GetMyErrorRate 普通用户：仅统计本人请求的错误率
func GetMyErrorRate(c *gin.Context) {
	userId := c.GetInt("id")
	window, start, end, bucketSize := resolveTimeRange(c)
	result, err := model.QueryErrorRate(userId, nil, false, window, start, end, bucketSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

// GetErrorRate 管理员：全局错误率，可按渠道/标签筛选
func GetErrorRate(c *gin.Context) {
	window, start, end, bucketSize := resolveTimeRange(c)
	channelIDs, applyFilter := resolveChannelFilter(c)
	result, err := model.QueryErrorRate(0, channelIDs, applyFilter, window, start, end, bucketSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
