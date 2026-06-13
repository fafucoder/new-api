// Package service — error_rate_alert: 错误率监控后台任务 + 告警发送。
//
// 后台 goroutine 周期性扫描所有 enabled 规则,查询指定范围和时间
// 窗口的错误率,超过阈值时通过 webhook 发出告警。状态翻转 + cooldown
// 控制告警频率:首次超标立即告警;持续超标按 cooldown 间隔重复告警;
// 错误率降低清状态,下次再超标又视为"新事件"立即告警。
package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	DefaultErrorRateAlertCooldownMinutes = 60
	errorRateAlertMinScanInterval        = time.Minute
)

// AutomaticallyAlertErrorRate 后台循环。在 main.go 用 `go` 起,
// 受 MonitorSetting.AutoErrorRateAlertEnabled / AutoErrorRateAlertMinutes
// 控制是否实际扫描。
func AutomaticallyAlertErrorRate() {
	common.SysLog("error rate alert auto-scan task started")
	for {
		setting := operation_setting.GetMonitorSetting()
		if !setting.AutoErrorRateAlertEnabled {
			time.Sleep(errorRateAlertMinScanInterval)
			continue
		}
		runErrorRateAlertScan()

		interval := setting.AutoErrorRateAlertMinutes
		if interval <= 0 {
			interval = 5 // 默认5分钟扫描一次
		}
		time.Sleep(time.Duration(interval * float64(time.Minute)))
	}
}

func runErrorRateAlertScan() {
	rules, err := model.ListEnabledErrorRateAlertRules()
	if err != nil {
		common.SysError("error rate alert: list rules failed: " + err.Error())
		return
	}
	for i := range rules {
		rule := &rules[i]
		if err := evaluateErrorRateAlertRule(rule); err != nil {
			common.SysError(fmt.Sprintf("error rate alert: rule %d eval failed: %s", rule.Id, err.Error()))
		}
	}
}

// evaluateErrorRateAlertRule 检查单条规则,按状态翻转 + cooldown 决
// 定是否发告警。错误率 >= threshold 触发;错误率 < threshold 清状态。
func evaluateErrorRateAlertRule(rule *model.ErrorRateAlertRule) error {
	// 计算时间范围
	windowSec, ok := model.ErrorRateAlertWindows[rule.TimeWindow]
	if !ok {
		return fmt.Errorf("invalid time window: %s", rule.TimeWindow)
	}
	now := time.Now().Unix()
	start := now - windowSec
	end := now

	// 根据 scope 查询错误率
	var result model.ErrorRateResult
	var err error
	var channelIDs []int
	applyChannelFilter := false

	switch rule.Scope {
	case model.ErrorRateAlertScopeGlobal:
		// 全局监控，不过滤渠道
		result, err = model.QueryErrorRate(0, nil, false, rule.TimeWindow, start, end, windowSec)
	case model.ErrorRateAlertScopeChannel:
		// 单渠道监控
		channelID, parseErr := strconv.Atoi(rule.ScopeValue)
		if parseErr != nil {
			return fmt.Errorf("invalid channel id: %s", rule.ScopeValue)
		}
		channelIDs = []int{channelID}
		applyChannelFilter = true
		result, err = model.QueryErrorRate(0, channelIDs, applyChannelFilter, rule.TimeWindow, start, end, windowSec)
	case model.ErrorRateAlertScopeTag:
		// 标签组监控：查询该 tag 下所有渠道
		channels, chErr := model.ListChannelsForTag(rule.ScopeValue)
		if chErr != nil {
			return fmt.Errorf("list channels for tag %s failed: %s", rule.ScopeValue, chErr.Error())
		}
		for _, ch := range channels {
			channelIDs = append(channelIDs, ch.Id)
		}
		if len(channelIDs) == 0 {
			// 该标签下没有渠道，记录为正常状态
			return model.MarkErrorRateAlertRuleScanned(rule.Id, 0, model.ErrorRateAlertStateNormal, false)
		}
		applyChannelFilter = true
		result, err = model.QueryErrorRate(0, channelIDs, applyChannelFilter, rule.TimeWindow, start, end, windowSec)
	default:
		return fmt.Errorf("invalid scope: %s", rule.Scope)
	}

	if err != nil {
		return err
	}

	errorRate := result.ErrorRate
	// 错误率低于阈值，清除告警状态
	if errorRate < rule.Threshold {
		return model.MarkErrorRateAlertRuleScanned(rule.Id, errorRate, model.ErrorRateAlertStateNormal, false)
	}

	// 错误率超过阈值，判断是否需要告警
	cooldownMinutes := operation_setting.GetMonitorSetting().ErrorRateAlertCooldownMinutes
	if cooldownMinutes <= 0 {
		cooldownMinutes = DefaultErrorRateAlertCooldownMinutes
	}
	cooldownSec := int64(cooldownMinutes * 60)
	shouldAlert := rule.AlertState != model.ErrorRateAlertStateAlerting ||
		rule.LastAlertedAt == 0 ||
		(now-rule.LastAlertedAt) >= cooldownSec

	if shouldAlert {
		if err := SendErrorRateAlert(rule, &result); err != nil {
			common.SysError(fmt.Sprintf("error rate alert: send for rule %d failed: %s", rule.Id, err.Error()))
			return model.MarkErrorRateAlertRuleScanned(rule.Id, errorRate, model.ErrorRateAlertStateAlerting, false)
		}
		return model.MarkErrorRateAlertRuleScanned(rule.Id, errorRate, model.ErrorRateAlertStateAlerting, true)
	}
	return model.MarkErrorRateAlertRuleScanned(rule.Id, errorRate, model.ErrorRateAlertStateAlerting, false)
}

// SendErrorRateAlert 发送错误率告警
func SendErrorRateAlert(rule *model.ErrorRateAlertRule, result *model.ErrorRateResult) error {
	message := fmt.Sprintf("【错误率告警】%s 内错误率达到 %.2f%%，超过阈值 %.2f%%",
		rule.TimeWindow, result.ErrorRate, rule.Threshold)

	// 构造告警内容
	content := fmt.Sprintf(
		"规则名称: %s\n"+
			"监控范围: %s\n"+
			"时间窗口: %s\n"+
			"错误率阈值: %.2f%%\n"+
			"当前错误率: %.2f%%\n"+
			"错误请求数: %d\n"+
			"成功请求数: %d\n"+
			"总请求数: %d",
		rule.Name,
		getScopeDisplay(rule.Scope, rule.ScopeValue),
		rule.TimeWindow,
		rule.Threshold,
		result.ErrorRate,
		result.ErrorCount,
		result.SuccessCount,
		result.Total,
	)

	// 使用规则自定义的 webhook 或系统默认
	webhookURL := rule.WebhookURL
	webhookSecret := rule.WebhookSecret
	if webhookURL == "" {
		// 使用系统默认 webhook
		setting := operation_setting.GetMonitorSetting()
		webhookURL = setting.WebhookURL
		webhookSecret = setting.WebhookSecret
	}

	if webhookURL != "" {
		notify := dto.NewNotify("error_rate_alert", message, content, nil)
		return SendWebhookNotify(webhookURL, webhookSecret, notify)
	}

	// 如果没有配置 webhook，记录到日志
	common.SysLog(message)
	return nil
}

// SendErrorRateAlertTest 发送测试错误率告警（带 [测试] 前缀）
func SendErrorRateAlertTest(rule *model.ErrorRateAlertRule, result *model.ErrorRateResult) error {
	message := fmt.Sprintf("[测试] 【错误率告警】%s 内错误率达到 %.2f%%，超过阈值 %.2f%%",
		rule.TimeWindow, result.ErrorRate, rule.Threshold)

	// 构造告警内容
	content := fmt.Sprintf(
		"规则名称: %s\n"+
			"监控范围: %s\n"+
			"时间窗口: %s\n"+
			"错误率阈值: %.2f%%\n"+
			"当前错误率: %.2f%%\n"+
			"错误请求数: %d\n"+
			"成功请求数: %d\n"+
			"总请求数: %d\n\n"+
			"这是一条测试告警消息",
		rule.Name,
		getScopeDisplay(rule.Scope, rule.ScopeValue),
		rule.TimeWindow,
		rule.Threshold,
		result.ErrorRate,
		result.ErrorCount,
		result.SuccessCount,
		result.Total,
	)

	// 使用规则自定义的 webhook 或系统默认
	webhookURL := rule.WebhookURL
	webhookSecret := rule.WebhookSecret
	if webhookURL == "" {
		// 使用系统默认 webhook
		setting := operation_setting.GetMonitorSetting()
		webhookURL = setting.WebhookURL
		webhookSecret = setting.WebhookSecret
	}

	if webhookURL != "" {
		notify := dto.NewNotify("error_rate_alert_test", message, content, nil)
		return SendWebhookNotify(webhookURL, webhookSecret, notify)
	}

	// 如果没有配置 webhook，记录到日志
	common.SysLog(message)
	return nil
}

func getScopeDisplay(scope, scopeValue string) string {
	switch scope {
	case model.ErrorRateAlertScopeGlobal:
		return "全局"
	case model.ErrorRateAlertScopeChannel:
		return fmt.Sprintf("渠道 #%s", scopeValue)
	case model.ErrorRateAlertScopeTag:
		return fmt.Sprintf("标签组 \"%s\"", scopeValue)
	default:
		return scope
	}
}
