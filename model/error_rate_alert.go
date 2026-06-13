// Package model — error_rate_alert: 错误率监控规则
//
// 监控指定范围(全局/渠道/标签)在给定时间窗口内的错误率,超过
// 阈值时通过 webhook 发出告警。支持状态翻转 + cooldown 抑制
// 重复告警。
package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ErrorRateAlertStateNormal   = "normal"   // 上次扫描错误率正常
	ErrorRateAlertStateAlerting = "alerting" // 上次扫描已告警,等待恢复

	ErrorRateAlertScopeGlobal  = "global"  // 全局监控
	ErrorRateAlertScopeChannel = "channel" // 单渠道监控
	ErrorRateAlertScopeTag     = "tag"     // 标签组监控
)

// ErrorRateAlertRule 持久化错误率监控规则。
// 每条规则定义监控范围、时间窗口、阈值和告警目标。
type ErrorRateAlertRule struct {
	Id            int     `json:"id" gorm:"primaryKey"`
	Name          string  `json:"name" gorm:"type:varchar(128)"`
	Scope         string  `json:"scope" gorm:"type:varchar(16);index"`           // global/channel/tag
	ScopeValue    string  `json:"scope_value" gorm:"type:varchar(64);index"`     // ""(global) / channel_id / tag
	TimeWindow    string  `json:"time_window" gorm:"type:varchar(8)"`            // 5m/15m/30m/1h/6h
	Threshold     float64 `json:"threshold"`                                     // 错误率阈值 (0-100, 百分比)
	WebhookURL    string  `json:"webhook_url" gorm:"type:varchar(512)"`          // 可选，留空使用系统默认
	WebhookSecret string  `json:"webhook_secret" gorm:"type:varchar(128)"`       // 可选
	Enabled       bool    `json:"enabled" gorm:"default:true;index"`
	Remark        string  `json:"remark" gorm:"type:varchar(256)"`
	LastErrorRate float64 `json:"last_error_rate"`    // 上次检查的错误率
	LastAlertedAt int64   `json:"last_alerted_at"`    // 最近一次告警的 unix 秒
	LastCheckedAt int64   `json:"last_checked_at"`    // 最近一次检查时间
	AlertState    string  `json:"alert_state" gorm:"type:varchar(16);default:'normal'"`
	CreatedTime   int64   `json:"created_time" gorm:"bigint;autoCreateTime"`
	UpdatedTime   int64   `json:"updated_time" gorm:"bigint;autoUpdateTime"`
}

func (ErrorRateAlertRule) TableName() string {
	return "error_rate_alert_rules"
}

// 时间窗口定义（秒）
var ErrorRateAlertWindows = map[string]int64{
	"5m":  300,
	"15m": 900,
	"30m": 1800,
	"1h":  3600,
	"6h":  21600,
}

// ValidateErrorRateAlertRule 校验规则字段
func ValidateErrorRateAlertRule(rule *ErrorRateAlertRule) error {
	if rule == nil {
		return errors.New("nil rule")
	}
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Name == "" {
		return errors.New("name is required")
	}
	if rule.Scope != ErrorRateAlertScopeGlobal &&
		rule.Scope != ErrorRateAlertScopeChannel &&
		rule.Scope != ErrorRateAlertScopeTag {
		return errors.New("invalid scope, must be global/channel/tag")
	}
	if rule.Scope != ErrorRateAlertScopeGlobal {
		rule.ScopeValue = strings.TrimSpace(rule.ScopeValue)
		if rule.ScopeValue == "" {
			return errors.New("scope_value is required for channel/tag scope")
		}
	}
	if _, ok := ErrorRateAlertWindows[rule.TimeWindow]; !ok {
		return errors.New("invalid time_window, must be 5m/15m/30m/1h/6h")
	}
	if rule.Threshold < 0 || rule.Threshold > 100 {
		return errors.New("threshold must be between 0 and 100")
	}
	if rule.AlertState == "" {
		rule.AlertState = ErrorRateAlertStateNormal
	}
	return nil
}

// CreateErrorRateAlertRule 创建新规则
func CreateErrorRateAlertRule(rule *ErrorRateAlertRule) error {
	if err := ValidateErrorRateAlertRule(rule); err != nil {
		return err
	}
	return DB.Create(rule).Error
}

// UpdateErrorRateAlertRule 更新规则字段（不包括运行时状态）
func UpdateErrorRateAlertRule(rule *ErrorRateAlertRule) error {
	if rule == nil || rule.Id <= 0 {
		return errors.New("invalid rule id")
	}
	if err := ValidateErrorRateAlertRule(rule); err != nil {
		return err
	}
	return DB.Model(&ErrorRateAlertRule{}).
		Where("id = ?", rule.Id).
		Updates(map[string]any{
			"name":           rule.Name,
			"scope":          rule.Scope,
			"scope_value":    rule.ScopeValue,
			"time_window":    rule.TimeWindow,
			"threshold":      rule.Threshold,
			"webhook_url":    rule.WebhookURL,
			"webhook_secret": rule.WebhookSecret,
			"enabled":        rule.Enabled,
			"remark":         rule.Remark,
		}).Error
}

// MarkErrorRateAlertRuleScanned 记录扫描结果
func MarkErrorRateAlertRuleScanned(id int, errorRate float64, state string, alertedNow bool) error {
	if id <= 0 {
		return errors.New("invalid rule id")
	}
	updates := map[string]any{
		"last_error_rate": errorRate,
		"last_checked_at": common.GetTimestamp(),
		"alert_state":     state,
	}
	if alertedNow {
		updates["last_alerted_at"] = common.GetTimestamp()
	}
	return DB.Model(&ErrorRateAlertRule{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteErrorRateAlertRule(id int) error {
	if id <= 0 {
		return errors.New("invalid rule id")
	}
	return DB.Delete(&ErrorRateAlertRule{}, id).Error
}

func GetErrorRateAlertRule(id int) (*ErrorRateAlertRule, error) {
	if id <= 0 {
		return nil, errors.New("invalid rule id")
	}
	var rule ErrorRateAlertRule
	if err := DB.First(&rule, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

// ListErrorRateAlertRules 返回所有规则（管理界面用）
func ListErrorRateAlertRules() ([]ErrorRateAlertRule, error) {
	var rules []ErrorRateAlertRule
	if err := DB.Order("id DESC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

// ListEnabledErrorRateAlertRules 返回所有启用的规则（后台扫描用）
func ListEnabledErrorRateAlertRules() ([]ErrorRateAlertRule, error) {
	var rules []ErrorRateAlertRule
	if err := DB.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}
