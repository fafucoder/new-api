package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// QuotaData 柱状图数据
type QuotaData struct {
	Id            int    `json:"id"`
	UserID        int    `json:"user_id" gorm:"index"`
	Username      string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName     string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2"`
	UseGroup      string `json:"use_group" gorm:"index;size:64;default:''"`
	TokenID       int    `json:"token_id" gorm:"index;default:0"`
	ChannelID     int    `json:"channel_id" gorm:"index;default:0"`
	NodeName      string `json:"node_name" gorm:"index;size:64;default:''"`
	TokenUsed     int    `json:"token_used" gorm:"default:0"`
	Count         int    `json:"count" gorm:"default:0"`
	Quota         int    `json:"quota" gorm:"default:0"`
	CacheHitCount int    `json:"cache_hit_count" gorm:"default:0"`
	CachedTokens  int    `json:"cached_tokens" gorm:"default:0"`
	InputTokens   int    `json:"input_tokens" gorm:"default:0"`
	// CostQuota is the channel cost (进货价) for these requests = quota *
	// channel_ratio, aggregated the same way as Quota (售价). No GORM default tag
	// to avoid cross-DB default churn; column added by AutoMigrate.
	CostQuota int `json:"cost_quota" gorm:"default:0"`
}

type QuotaDataLogParams struct {
	UserID       int
	Username     string
	ModelName    string
	Quota        int
	CreatedAt    int64
	TokenUsed    int
	UseGroup     string
	TokenID      int
	ChannelID    int
	NodeName     string
	CachedTokens int
	InputTokens  int
	CostQuota    int
}

func UpdateQuotaData() {
	for {
		if common.DataExportEnabled {
			common.SysLog("正在更新数据看板数据...")
			SaveQuotaDataCache()
		}
		time.Sleep(time.Duration(common.DataExportInterval) * time.Minute)
	}
}

var CacheQuotaData = make(map[string]*QuotaData)
var CacheQuotaDataLock = sync.Mutex{}

func logQuotaDataCache(quotaData *QuotaData) {
	key := fmt.Sprintf("%d\x00%s\x00%s\x00%d\x00%s\x00%d\x00%d\x00%s",
		quotaData.UserID,
		quotaData.Username,
		quotaData.ModelName,
		quotaData.CreatedAt,
		quotaData.UseGroup,
		quotaData.TokenID,
		quotaData.ChannelID,
		quotaData.NodeName,
	)
	count := quotaData.Count
	quota := quotaData.Quota
	tokenUsed := quotaData.TokenUsed
	cachedQuotaData, ok := CacheQuotaData[key]
	if ok {
		cachedQuotaData.Count += count
		cachedQuotaData.Quota += quota
		cachedQuotaData.TokenUsed += tokenUsed
		cachedQuotaData.CacheHitCount += quotaData.CacheHitCount
		cachedQuotaData.CachedTokens += quotaData.CachedTokens
		cachedQuotaData.InputTokens += quotaData.InputTokens
		cachedQuotaData.CostQuota += quotaData.CostQuota
		quotaData = cachedQuotaData
	}
	CacheQuotaData[key] = quotaData
}

func LogQuotaData(params QuotaDataLogParams) {
	// 只精确到小时
	createdAt := params.CreatedAt - (params.CreatedAt % 3600)
	cacheHitCount := 0
	if params.CachedTokens > 0 {
		cacheHitCount = 1
	}
	quotaData := &QuotaData{
		UserID:        params.UserID,
		Username:      params.Username,
		ModelName:     params.ModelName,
		CreatedAt:     createdAt,
		UseGroup:      params.UseGroup,
		TokenID:       params.TokenID,
		ChannelID:     params.ChannelID,
		NodeName:      params.NodeName,
		Count:         1,
		Quota:         params.Quota,
		TokenUsed:     params.TokenUsed,
		CacheHitCount: cacheHitCount,
		CachedTokens:  params.CachedTokens,
		InputTokens:   params.InputTokens,
		CostQuota:     params.CostQuota,
	}

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	logQuotaDataCache(quotaData)
}

func SaveQuotaDataCache() {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	size := len(CacheQuotaData)
	// 如果缓存中有数据，就保存到数据库中
	// 1. 先查询数据库中是否有数据
	// 2. 如果有数据，就更新数据
	// 3. 如果没有数据，就插入数据
	for _, quotaData := range CacheQuotaData {
		quotaDataDB := &QuotaData{}
		DB.Table("quota_data").
			Where("user_id = ? and username = ? and model_name = ? and created_at = ? and use_group = ? and token_id = ? and channel_id = ? and node_name = ?",
				quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.CreatedAt, quotaData.UseGroup, quotaData.TokenID, quotaData.ChannelID, quotaData.NodeName).
			First(quotaDataDB)
		if quotaDataDB.Id > 0 {
			//quotaDataDB.Count += quotaData.Count
			//quotaDataDB.Quota += quotaData.Quota
			//DB.Table("quota_data").Save(quotaDataDB)
			increaseQuotaData(quotaData)
		} else {
			DB.Table("quota_data").Create(quotaData)
		}
	}
	CacheQuotaData = make(map[string]*QuotaData)
	common.SysLog(fmt.Sprintf("保存数据看板数据成功，共保存%d条数据", size))
}

func increaseQuotaData(quotaData *QuotaData) {
	err := DB.Table("quota_data").
		Where("user_id = ? and username = ? and model_name = ? and created_at = ? and use_group = ? and token_id = ? and channel_id = ? and node_name = ?",
			quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.CreatedAt, quotaData.UseGroup, quotaData.TokenID, quotaData.ChannelID, quotaData.NodeName).
		Updates(map[string]interface{}{
			"count":           gorm.Expr("count + ?", quotaData.Count),
			"quota":           gorm.Expr("quota + ?", quotaData.Quota),
			"token_used":      gorm.Expr("token_used + ?", quotaData.TokenUsed),
			"cache_hit_count": gorm.Expr("cache_hit_count + ?", quotaData.CacheHitCount),
			"cached_tokens":   gorm.Expr("cached_tokens + ?", quotaData.CachedTokens),
			"input_tokens":    gorm.Expr("input_tokens + ?", quotaData.InputTokens),
			"cost_quota":      gorm.Expr("cost_quota + ?", quotaData.CostQuota),
		}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("increaseQuotaData error: %s", err))
	}
}

func GetQuotaDataByUsername(username string, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").
		Select("user_id, username, model_name, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(cache_hit_count) as cache_hit_count, sum(cached_tokens) as cached_tokens, sum(input_tokens) as input_tokens").
		Where("username = ? and created_at >= ? and created_at <= ?", username, startTime, endTime).
		Group("user_id, username, model_name, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataByUserId(userId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	err = DB.Table("quota_data").
		Select("user_id, username, model_name, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(cache_hit_count) as cache_hit_count, sum(cached_tokens) as cached_tokens, sum(input_tokens) as input_tokens").
		Where("user_id = ? and created_at >= ? and created_at <= ?", userId, startTime, endTime).
		Group("user_id, username, model_name, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetQuotaDataGroupByUser(startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	var quotaDatas []*QuotaData
	err = DB.Table("quota_data").
		Select("username, created_at, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(cache_hit_count) as cache_hit_count, sum(cached_tokens) as cached_tokens, sum(input_tokens) as input_tokens").
		Where("created_at >= ? and created_at <= ?", startTime, endTime).
		Group("username, created_at").
		Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetAllQuotaDates(startTime int64, endTime int64, username string) (quotaData []*QuotaData, err error) {
	if username != "" {
		return GetQuotaDataByUsername(username, startTime, endTime)
	}
	var quotaDatas []*QuotaData
	// 从quota_data表中查询数据
	// only select model_name, sum(count) as count, sum(quota) as quota, model_name, created_at from quota_data group by model_name, created_at;
	//err = DB.Table("quota_data").Where("created_at >= ? and created_at <= ?", startTime, endTime).Find(&quotaDatas).Error
	err = DB.Table("quota_data").Select("model_name, sum(count) as count, sum(quota) as quota, sum(token_used) as token_used, sum(cache_hit_count) as cache_hit_count, sum(cached_tokens) as cached_tokens, sum(input_tokens) as input_tokens, created_at").Where("created_at >= ? and created_at <= ?", startTime, endTime).Group("model_name, created_at").Find(&quotaDatas).Error
	return quotaDatas, err
}

func GetCacheQuotaData(startTime int64, endTime int64, modelName string, channelID int) ([]*QuotaData, error) {
	var quotaDatas []*QuotaData
	tx := DB.Table("quota_data").
		Select("model_name, channel_id, created_at, sum(count) as count, sum(cache_hit_count) as cache_hit_count, sum(cached_tokens) as cached_tokens, sum(input_tokens) as input_tokens").
		Where("created_at >= ? and created_at <= ?", startTime, endTime)
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if channelID > 0 {
		tx = tx.Where("channel_id = ?", channelID)
	}
	err := tx.Group("model_name, channel_id, created_at").Find(&quotaDatas).Error
	return quotaDatas, err
}

// GetChannelCostData aggregates per-channel sale price (售价 = quota) and cost
// (进货价 = cost_quota) over the given time range, grouped by channel and hour
// bucket. An optional channelID > 0 narrows to a single channel. The frontend
// rolls the hourly buckets up into the requested time granularity and renders
// one bar group per channel.
func GetChannelCostData(startTime int64, endTime int64, channelID int) ([]*QuotaData, error) {
	var quotaDatas []*QuotaData
	tx := DB.Table("quota_data").
		Select("channel_id, created_at, sum(count) as count, sum(quota) as quota, sum(cost_quota) as cost_quota").
		Where("created_at >= ? and created_at <= ?", startTime, endTime)
	if channelID > 0 {
		tx = tx.Where("channel_id = ?", channelID)
	}
	err := tx.Group("channel_id, created_at").Find(&quotaDatas).Error
	return quotaDatas, err
}
