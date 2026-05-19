// generic.go: 自定义 HTTP webhook 适配器 (没命中任何已知平台时的兜底)。
//
// 用途: 用户自有服务、Apprise/ntfy 之类的通用转发器、或想自己实现签名校验
// 的接收端。schema 是 {type,title,content,values,timestamp}, 跟旧版
// service.SendWebhookNotify 保持一致, 老对接方不用改。
//
// 可选 secret 启用签名: HMAC-SHA256(secret, body) → hex, 走 X-Webhook-Signature
// 头 (接收方拿同 secret 校验请求未篡改); Worker 模式额外带 Authorization
// header, 保留旧行为。

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// genericPayload 通用 webhook 负载, 保持跟旧版 schema 一致。
type genericPayload struct {
	Type      string        `json:"type"`
	Title     string        `json:"title"`
	Content   string        `json:"content"`
	Values    []interface{} `json:"values,omitempty"`
	Timestamp int64         `json:"timestamp"`
}

// genericSignature 通用 webhook 签名: HMAC-SHA256(secret, body) → hex。
// 飞书/钉钉自己的签名算法在各自适配器里。
func genericSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func sendGeneric(webhookURL string, secret string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	payload := genericPayload{
		Type:      data.Type,
		Title:     data.Title,
		Content:   content,
		Values:    data.Values,
		Timestamp: time.Now().Unix(),
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %v", err)
	}
	var headers map[string]string
	if secret != "" {
		headers = map[string]string{
			"X-Webhook-Signature": genericSignature(secret, body),
			"Authorization":       "Bearer " + secret,
		}
	}
	_, err = post(webhookURL, body, headers)
	return err
}
