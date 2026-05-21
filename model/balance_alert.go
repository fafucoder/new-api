// Package model — balance_alert: 余额监控规则
//
// 多个渠道可能指向同一上游账户(共用一份额度)。按 Channel.Tag 把
// 这些渠道聚合成一个"上游账户",每个 tag 对应一条 BalanceAlertRule:
// 累计充值总额(TotalQuota) / 告警阈值 / 可选的 webhook 覆盖 /
// 上次告警时间 / 状态翻转标记。
//
// 计算"剩余 = TotalQuota - SUM(channels.used_quota / QuotaPerUnit)"。
// TotalQuota 由管理员手动维护(创建/编辑/充值);used_quota 由
// 网关请求计费链路自动累加,无需轮询上游 API,所以对所有渠道
// 类型(包括没有公开余额查询接口的)都有效。
package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	BalanceAlertStateNormal   = "normal"   // 上次扫描余额充足
	BalanceAlertStateAlerting = "alerting" // 上次扫描已告警,等待恢复
)

// BalanceAlertRule 持久化每个上游(以 Channel.Tag 标识)的余额监控规则。
// Tag 留作业务 key,唯一索引避免同上游配多条冲突规则。
type BalanceAlertRule struct {
	Id            int     `json:"id" gorm:"primaryKey"`
	Tag           string  `json:"tag" gorm:"type:varchar(64);uniqueIndex"`
	TotalQuota    float64 `json:"total_quota"`   // 累计充值总额 (USD)。计算剩余 = TotalQuota - 已用
	Threshold     float64 `json:"threshold"`     // 剩余低于此值触发告警,单位 USD
	WebhookURL    string  `json:"webhook_url" gorm:"type:varchar(512)"`
	WebhookSecret string  `json:"webhook_secret" gorm:"type:varchar(128)"`
	Enabled       bool    `json:"enabled" gorm:"default:true"`
	Remark        string  `json:"remark" gorm:"type:varchar(256)"`
	LastBalance   float64 `json:"last_balance"`    // 上次扫描记录的"剩余"
	LastAlertedAt int64   `json:"last_alerted_at"` // 最近一次发出告警的 unix 秒
	LastCheckedAt int64   `json:"last_checked_at"` // 最近一次扫描时间
	AlertState    string  `json:"alert_state" gorm:"type:varchar(16);default:'normal'"`
	CreatedTime   int64   `json:"created_time" gorm:"bigint;autoCreateTime"`
	UpdatedTime   int64   `json:"updated_time" gorm:"bigint;autoUpdateTime"`
}

func (BalanceAlertRule) TableName() string {
	return "balance_alert_rules"
}

// CreateBalanceAlertRule 写入新规则。tag/threshold 必填;
// tag 去前后空白以避免不一致。TotalQuota 可选,默认 0。
func CreateBalanceAlertRule(rule *BalanceAlertRule) error {
	if rule == nil {
		return errors.New("nil rule")
	}
	rule.Tag = strings.TrimSpace(rule.Tag)
	if rule.Tag == "" {
		return errors.New("tag is required")
	}
	if rule.Threshold < 0 {
		return errors.New("threshold must be >= 0")
	}
	if rule.TotalQuota < 0 {
		return errors.New("total_quota must be >= 0")
	}
	if rule.AlertState == "" {
		rule.AlertState = BalanceAlertStateNormal
	}
	return DB.Create(rule).Error
}

// UpdateBalanceAlertRule 更新规则字段。LastBalance/LastAlertedAt/
// LastCheckedAt/AlertState 由后台扫描任务负责,这里只更新管理员
// 可编辑的字段,避免界面操作覆盖运行时状态。
// 注意 TotalQuota 也支持编辑(纠错场景);常规"加钱"请走 TopupBalanceAlertRule
// 以保持原子性。
func UpdateBalanceAlertRule(rule *BalanceAlertRule) error {
	if rule == nil || rule.Id <= 0 {
		return errors.New("invalid rule id")
	}
	rule.Tag = strings.TrimSpace(rule.Tag)
	if rule.Tag == "" {
		return errors.New("tag is required")
	}
	if rule.Threshold < 0 {
		return errors.New("threshold must be >= 0")
	}
	if rule.TotalQuota < 0 {
		return errors.New("total_quota must be >= 0")
	}
	return DB.Model(&BalanceAlertRule{}).
		Where("id = ?", rule.Id).
		Updates(map[string]any{
			"tag":            rule.Tag,
			"total_quota":    rule.TotalQuota,
			"threshold":      rule.Threshold,
			"webhook_url":    rule.WebhookURL,
			"webhook_secret": rule.WebhookSecret,
			"enabled":        rule.Enabled,
			"remark":         rule.Remark,
		}).Error
}

// TopupBalanceAlertRule 原子地给规则的 TotalQuota 加上 amount(累计充值)。
// 同时把 AlertState 翻回 normal,这样"充值后再次跌破阈值"能立即重新告警,
// 不被原 cooldown 抑制。
func TopupBalanceAlertRule(id int, amount float64) error {
	if id <= 0 {
		return errors.New("invalid rule id")
	}
	if amount <= 0 {
		return errors.New("amount must be > 0")
	}
	return DB.Model(&BalanceAlertRule{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"total_quota": gorm.Expr("total_quota + ?", amount),
			"alert_state": BalanceAlertStateNormal,
		}).Error
}

// MarkBalanceAlertRuleScanned 由后台扫描任务调用,记录最新观察到
// 的剩余余额(remaining)和告警状态。单独的方法避免 Updates 覆盖管理员字段。
func MarkBalanceAlertRuleScanned(id int, balance float64, state string, alertedNow bool) error {
	if id <= 0 {
		return errors.New("invalid rule id")
	}
	updates := map[string]any{
		"last_balance":    balance,
		"last_checked_at": common.GetTimestamp(),
		"alert_state":     state,
	}
	if alertedNow {
		updates["last_alerted_at"] = common.GetTimestamp()
	}
	return DB.Model(&BalanceAlertRule{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteBalanceAlertRule(id int) error {
	if id <= 0 {
		return errors.New("invalid rule id")
	}
	return DB.Delete(&BalanceAlertRule{}, id).Error
}

func GetBalanceAlertRule(id int) (*BalanceAlertRule, error) {
	if id <= 0 {
		return nil, errors.New("invalid rule id")
	}
	var rule BalanceAlertRule
	if err := DB.First(&rule, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

// ListBalanceAlertRules 返回所有规则,按 tag 字典序。给管理界面用,
// 数量级很小(等于"上游账户数"),不分页。
func ListBalanceAlertRules() ([]BalanceAlertRule, error) {
	var rules []BalanceAlertRule
	if err := DB.Order("tag ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// ListEnabledBalanceAlertRules 给后台扫描任务用,跳过已禁用规则。
func ListEnabledBalanceAlertRules() ([]BalanceAlertRule, error) {
	var rules []BalanceAlertRule
	if err := DB.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// TagBalanceSummary 是按 tag 聚合的余额快照,给前端表格展示。
// Balance = TotalQuota - UsedUSD (剩余可用,USD)。
type TagBalanceSummary struct {
	Tag                string  `json:"tag"`
	TotalQuota         float64 `json:"total_quota"`          // 累计充值总额 (USD) — 从规则带过来便于前端单点取值
	UsedUSD            float64 `json:"used_usd"`             // 已用 (USD) = SUM(channel.used_quota for tag) / QuotaPerUnit
	Balance            float64 `json:"balance"`              // 剩余 = TotalQuota - UsedUSD
	BalanceUpdatedTime int64   `json:"balance_updated_time"` // 字段保留以兼容旧调用方;现在固定取扫描时间或 0
	ChannelCount       int     `json:"channel_count"`        // 同 tag 渠道数
	EnabledCount       int     `json:"enabled_count"`        // 其中处于 enabled 状态的数量
}

// AggregateBalanceForTag 按 tag 聚合得到剩余余额:
// 累计已用 = SUM(channel.used_quota) / common.QuotaPerUnit
// 剩余    = rule.TotalQuota - 累计已用
// rule 可能为 nil(只有 channels 没有规则),此时 TotalQuota 视为 0,
// Balance 直接显示为负的"已用",给管理员一个直观提示需要创建规则。
func AggregateBalanceForTag(tag string) (*TagBalanceSummary, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, errors.New("tag is required")
	}
	var channels []Channel
	err := DB.Select("id", "name", "status", "used_quota").
		Where("tag = ?", tag).
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	summary := &TagBalanceSummary{Tag: tag, ChannelCount: len(channels)}
	var totalUsedQuota int64
	for _, ch := range channels {
		if ch.Status == common.ChannelStatusEnabled {
			summary.EnabledCount++
		}
		totalUsedQuota += ch.UsedQuota
	}
	if common.QuotaPerUnit > 0 {
		summary.UsedUSD = float64(totalUsedQuota) / common.QuotaPerUnit
	}

	// 把规则的 TotalQuota 带进来一起返回。规则不存在(用户还没建)时
	// TotalQuota = 0,Balance = -UsedUSD,前端能据此提示"未配置总额度"。
	var rule BalanceAlertRule
	if err := DB.Select("total_quota").Where("tag = ?", tag).Take(&rule).Error; err == nil {
		summary.TotalQuota = rule.TotalQuota
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	summary.Balance = summary.TotalQuota - summary.UsedUSD
	return summary, nil
}

// ListChannelTags 返回 channels 表里所有非空 tag(去重),给前端
// 「新建规则」时下拉选择用。
func ListChannelTags() ([]string, error) {
	var tags []string
	err := DB.Model(&Channel{}).
		Where("tag IS NOT NULL AND tag != ''").
		Distinct("tag").
		Order("tag ASC").
		Pluck("tag", &tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

// ListChannelsForTag 返回某 tag 下的渠道清单(只取展示需要的字段),
// 给监控详情/webhook payload 用。
func ListChannelsForTag(tag string) ([]Channel, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, errors.New("tag is required")
	}
	var channels []Channel
	err := DB.Select("id", "name", "status", "balance", "balance_updated_time", "used_quota", "type").
		Where("tag = ?", tag).
		Order("id ASC").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}
