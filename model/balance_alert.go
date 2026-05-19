// Package model — balance_alert: 余额监控规则
//
// 多个渠道可能指向同一上游账户(共用一份额度)。按 Channel.Tag 把
// 这些渠道聚合成一个"上游账户",每个 tag 对应一条 BalanceAlertRule:
// 阈值 / 可选的 webhook 覆盖 / 上次告警时间 / 状态翻转标记。
//
// 规则表只持久化"应该怎么告警"。聚合余额是查询时从 channels 表实时
// 算出来的,不在这里冗余存储 — 渠道 balance 由现有的
// AutomaticallyUpdateChannels 任务负责刷新。
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
	Threshold     float64 `json:"threshold"`     // 余额低于此值触发,单位 USD
	WebhookURL    string  `json:"webhook_url" gorm:"type:varchar(512)"`
	WebhookSecret string  `json:"webhook_secret" gorm:"type:varchar(128)"`
	Enabled       bool    `json:"enabled" gorm:"default:true"`
	Remark        string  `json:"remark" gorm:"type:varchar(256)"`
	LastBalance   float64 `json:"last_balance"`    // 上次扫描记录的余额
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
// tag 去前后空白以避免不一致。
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
	if rule.AlertState == "" {
		rule.AlertState = BalanceAlertStateNormal
	}
	return DB.Create(rule).Error
}

// UpdateBalanceAlertRule 更新规则字段。LastBalance/LastAlertedAt/
// LastCheckedAt/AlertState 由后台扫描任务负责,这里只更新管理员
// 可编辑的字段,避免界面操作覆盖运行时状态。
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
	return DB.Model(&BalanceAlertRule{}).
		Where("id = ?", rule.Id).
		Updates(map[string]any{
			"tag":            rule.Tag,
			"threshold":      rule.Threshold,
			"webhook_url":    rule.WebhookURL,
			"webhook_secret": rule.WebhookSecret,
			"enabled":        rule.Enabled,
			"remark":         rule.Remark,
		}).Error
}

// MarkBalanceAlertRuleScanned 由后台扫描任务调用,记录最新观察到
// 的余额和告警状态。单独的方法避免 Updates 覆盖管理员字段。
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
type TagBalanceSummary struct {
	Tag                string  `json:"tag"`
	Balance            float64 `json:"balance"`              // 同 tag 渠道里 balance_updated_time 最新的一个值
	BalanceUpdatedTime int64   `json:"balance_updated_time"` // 上面那个 balance 对应的更新时间
	ChannelCount       int     `json:"channel_count"`        // 同 tag 渠道数
	EnabledCount       int     `json:"enabled_count"`        // 其中处于 enabled 状态的数量
}

// AggregateBalanceForTag 取同 tag 渠道里 balance_updated_time 最新
// 的一个余额作为该上游的当前余额。同 tag 渠道余额本应一致(同一
// 上游账户),取最新值是为了避免老 channel 拉取失败后的过期数据
// 干扰判定。
func AggregateBalanceForTag(tag string) (*TagBalanceSummary, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, errors.New("tag is required")
	}
	var channels []Channel
	err := DB.Select("id", "name", "status", "balance", "balance_updated_time").
		Where("tag = ?", tag).
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	summary := &TagBalanceSummary{Tag: tag, ChannelCount: len(channels)}
	for _, ch := range channels {
		if ch.Status == common.ChannelStatusEnabled {
			summary.EnabledCount++
		}
		if ch.BalanceUpdatedTime > summary.BalanceUpdatedTime {
			summary.BalanceUpdatedTime = ch.BalanceUpdatedTime
			summary.Balance = ch.Balance
		}
	}
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
	err := DB.Select("id", "name", "status", "balance", "balance_updated_time", "type").
		Where("tag = ?", tag).
		Order("id ASC").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}
