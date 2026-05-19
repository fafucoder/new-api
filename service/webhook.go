// Package service — webhook.go: 通知 webhook 入口的 service 层薄壳。
//
// 真实逻辑都在 service/webhook 子包里 (按 URL 自动选飞书/钉钉/企微/通用
// 适配器)。这层 wrapper 只是为了让老调用方 (service/user_notify.go,
// service/balance_alert.go) 不用改 import 路径 — 直接 service.SendWebhookNotify
// 就行。新代码建议直接 import "github.com/QuantumNous/new-api/service/webhook"
// 调 webhook.Send。

package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/webhook"
)

func SendWebhookNotify(webhookURL string, secret string, data dto.Notify) error {
	return webhook.Send(webhookURL, secret, data)
}
