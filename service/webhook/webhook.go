// Package webhook 提供通知 webhook 的多平台适配。
//
// 架构: 接口 + 注册表 (见 adapter.go)。每个平台一个文件, 实现 Adapter 后
// 在 init() 里 Register, Send 自动通过注册表 dispatch — 加新平台不用动
// dispatch 逻辑。包内禁止反向依赖 service, 否则会形成循环导入
// (service/balance_alert.go 等会调到这里)。
//
// 已内置适配器 (按 URL 自动选, 互斥不重叠):
//
//	国内
//	  open.feishu.cn / open.larksuite.com → 飞书 / Lark      (feishu.go)
//	  oapi.dingtalk.com                   → 钉钉             (dingtalk.go)
//	  qyapi.weixin.qq.com                 → 企业微信          (wecom.go)
//
//	国外
//	  api.telegram.org/bot                → Telegram         (telegram.go)
//	  hooks.slack.com/services            → Slack            (slack.go)
//	  discord.com|discordapp.com/api/webhooks → Discord       (discord.go)
//	  *.webhook.office.com/webhookb2      → Microsoft Teams  (teams.go)
//
//	兜底
//	  其它任何 URL                          → 通用 HTTP       (generic.go)
//
// 兜底的 generic 不进注册表 — 它是显式 fallback, 没有 Match 概念, 任何
// URL 都能投, 用通用 schema {type,title,content,timestamp}。
package webhook

import (
	"github.com/QuantumNous/new-api/dto"
)

// Send 把通知投递到 webhookURL。注册表里命中的适配器走专用通道, 否则
// 按通用 HTTP schema POST (sendGeneric)。secret 含义按各平台规则:
//
//   - generic:  HMAC-SHA256(secret, body) → hex, 写 X-Webhook-Signature 头
//   - feishu:   计算 timestamp+sign 放进 body
//   - dingtalk: 计算 timestamp+sign 拼到 URL query
//   - wecom:    不使用 (鉴权用 URL 上的 key)
//   - telegram: chat_id (URL 没 ?chat_id= 时从这里取)
//   - slack:    不使用 (URL 路径里带 token)
//   - discord:  不使用 (URL 路径里带 token)
//   - teams:    不使用 (URL 路径里带随机 path)
func Send(webhookURL string, secret string, data dto.Notify) error {
	if a := pickAdapter(webhookURL); a != nil {
		return a.Send(webhookURL, secret, data)
	}
	return sendGeneric(webhookURL, secret, data)
}
