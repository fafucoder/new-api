// telegram.go: Telegram Bot API 适配器。
//
// Telegram 的 sendMessage 需要两样东西: bot token + chat_id。token 已经在
// URL 路径里 (/bot<TOKEN>/sendMessage), 缺的只有 chat_id, 我们按以下优先级
// 解析:
//
//  1. URL query 里的 ?chat_id=...  (用户能拼完整 URL 直接粘进来)
//  2. secret 字段                  (用户没把 chat_id 写进 URL 时用这里)
//
// 两处都没有就直接报错 — 否则消息发出去也会被 Telegram 静默拒绝。
//
// schema: {chat_id, text}; 响应 {ok, error_code, description}, ok=false
// 时把 description 透到 error 里给上层定位。
// 文档: https://core.telegram.org/bots/api#sendmessage

package webhook

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func init() {
	Register(telegramAdapter{})
}

type telegramAdapter struct{}

func (telegramAdapter) Name() string { return "telegram" }

// Match 命中 https://api.telegram.org/bot<TOKEN>/sendMessage
func (telegramAdapter) Match(webhookURL string) bool {
	u := strings.ToLower(webhookURL)
	return strings.Contains(u, "api.telegram.org/bot")
}

type telegramPayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

func (telegramAdapter) Send(webhookURL string, secret string, data dto.Notify) error {
	parsed, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid telegram url: %v", err)
	}
	chatID := parsed.Query().Get("chat_id")
	if chatID == "" {
		chatID = strings.TrimSpace(secret)
	}
	if chatID == "" {
		return fmt.Errorf("telegram webhook 需要 chat_id (URL 加 ?chat_id=... 或填到 secret 字段)")
	}

	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	text := content
	if strings.TrimSpace(data.Title) != "" {
		text = data.Title + "\n\n" + content
	}

	payload := telegramPayload{
		ChatID: chatID,
		Text:   text,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %v", err)
	}
	respBytes, err := post(webhookURL, body, nil)
	if err != nil {
		return err
	}
	var resp struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code,omitempty"`
		Description string `json:"description,omitempty"`
	}
	if err := common.Unmarshal(respBytes, &resp); err == nil && !resp.OK {
		return fmt.Errorf("telegram rejected message: error_code=%d description=%s", resp.ErrorCode, resp.Description)
	}
	return nil
}
