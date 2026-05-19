// Package service — balance_alert: 余额监控后台任务 + 告警发送。
//
// 后台 goroutine 周期性扫描所有 enabled 规则,聚合 tag 下渠道的
// 余额,跌破阈值时通过 webhook(规则自带)或 NotifyRootUser 发出
// 告警。状态翻转 + cooldown 控制 spam:第一次跌破立即告警;持续
// 低余额按 cooldown 间隔重复告警;余额回升清状态,下次再跌破又
// 视为"新事件"立即告警。
package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	// DefaultBalanceAlertCooldownHours 持续低余额时的告警冷却,
	// 防止规则一直跌破阈值就一直发。
	DefaultBalanceAlertCooldownHours = 6
	// balanceAlertMinScanInterval 配置异常或暂关时的轮询心跳,
	// 让"刚开启就生效"不需要等到下一个长间隔。
	balanceAlertMinScanInterval = time.Minute
)

// AutomaticallyAlertBalance 后台循环。在 main.go 用 `go` 起,
// 受 MonitorSetting.AutoBalanceAlertEnabled / AutoBalanceAlertMinutes
// 控制是否实际扫描。
func AutomaticallyAlertBalance() {
	common.SysLog("balance alert auto-scan task started")
	for {
		setting := operation_setting.GetMonitorSetting()
		if !setting.AutoBalanceAlertEnabled {
			time.Sleep(balanceAlertMinScanInterval)
			continue
		}
		runBalanceAlertScan()

		interval := setting.AutoBalanceAlertMinutes
		if interval <= 0 {
			interval = 30
		}
		time.Sleep(time.Duration(interval * float64(time.Minute)))
	}
}

func runBalanceAlertScan() {
	rules, err := model.ListEnabledBalanceAlertRules()
	if err != nil {
		common.SysError("balance alert: list rules failed: " + err.Error())
		return
	}
	for i := range rules {
		rule := &rules[i]
		if err := evaluateBalanceAlertRule(rule); err != nil {
			common.SysError(fmt.Sprintf("balance alert: rule %d eval failed: %s", rule.Id, err.Error()))
		}
	}
}

// evaluateBalanceAlertRule 检查单条规则,按状态翻转 + cooldown 决
// 定是否发告警。balance < threshold 触发;balance >= threshold 清
// 状态。无渠道(tag 不命中)按 normal 处理,避免无意义告警。
func evaluateBalanceAlertRule(rule *model.BalanceAlertRule) error {
	summary, err := model.AggregateBalanceForTag(rule.Tag)
	if err != nil {
		return err
	}
	if summary == nil || summary.ChannelCount == 0 {
		return model.MarkBalanceAlertRuleScanned(rule.Id, 0, model.BalanceAlertStateNormal, false)
	}

	balance := summary.Balance
	if balance >= rule.Threshold {
		return model.MarkBalanceAlertRuleScanned(rule.Id, balance, model.BalanceAlertStateNormal, false)
	}

	cooldownHours := operation_setting.GetMonitorSetting().BalanceAlertCooldownHours
	if cooldownHours <= 0 {
		cooldownHours = DefaultBalanceAlertCooldownHours
	}
	cooldownSec := int64(cooldownHours * 3600)
	now := time.Now().Unix()
	shouldAlert := rule.AlertState != model.BalanceAlertStateAlerting ||
		rule.LastAlertedAt == 0 ||
		(now-rule.LastAlertedAt) >= cooldownSec

	if shouldAlert {
		if err := SendBalanceAlert(rule, balance, false); err != nil {
			common.SysError(fmt.Sprintf("balance alert: send for rule %d failed: %s", rule.Id, err.Error()))
			return model.MarkBalanceAlertRuleScanned(rule.Id, balance, model.BalanceAlertStateAlerting, false)
		}
		return model.MarkBalanceAlertRuleScanned(rule.Id, balance, model.BalanceAlertStateAlerting, true)
	}
	return model.MarkBalanceAlertRuleScanned(rule.Id, balance, model.BalanceAlertStateAlerting, false)
}

// SendBalanceAlert 发出一次告警。规则配了 webhook_url 走该 webhook,
// 否则 fallback 到 NotifyRootUser(走 root 配置的 email/webhook/bark
// 等通知方式)。isTest 标记仅影响标题,不更新规则状态。
func SendBalanceAlert(rule *model.BalanceAlertRule, balance float64, isTest bool) error {
	if rule == nil {
		return fmt.Errorf("nil rule")
	}
	subject := fmt.Sprintf("【余额预警】上游 %s 余额不足", rule.Tag)
	if isTest {
		subject = "[测试] " + subject
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "上游标签: %s\n", rule.Tag)
	fmt.Fprintf(&sb, "当前余额: $%.4f\n", balance)
	fmt.Fprintf(&sb, "告警阈值: $%.4f\n", rule.Threshold)
	if rule.Remark != "" {
		fmt.Fprintf(&sb, "备注: %s\n", rule.Remark)
	}
	fmt.Fprintf(&sb, "触发时间: %s", time.Now().Format("2006-01-02 15:04:05"))
	content := sb.String()

	notify := dto.NewNotify(dto.NotifyTypeBalanceLow, subject, content, nil)
	if strings.TrimSpace(rule.WebhookURL) != "" {
		return SendWebhookNotify(rule.WebhookURL, rule.WebhookSecret, notify)
	}
	// Fallback 到 root 用户的通知配置。直接调用 NotifyUser 以便把
	// "邮箱未配置 / 限频 / webhook 失败" 等错误透出来 — 测试入口靠
	// 它来告诉管理员真实失败原因, NotifyRootUser 会吞掉错误。
	rootUser := model.GetRootUser()
	if rootUser == nil {
		return fmt.Errorf("root user not found")
	}
	base := rootUser.ToBaseUser()
	return NotifyUser(base.Id, base.Email, base.GetSetting(), notify)
}
