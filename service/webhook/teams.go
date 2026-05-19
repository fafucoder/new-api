// teams.go: Microsoft Teams Incoming Webhook 适配器。
//
// 用旧的 Office 365 Connector "MessageCard" schema。微软在 2025 年底起
// 把 Connector 推到 Workflows, 但 MessageCard 仍然支持且对接最简单 —
// 等下游量大了再切 Adaptive Card 也来得及。
//
// 必须带 @type / @context, 否则 Teams 会 400。成功时返 HTTP 200 + 字符
// 串 "1" (不是 JSON), 不需要解 body — post() 处理掉非 2xx 就够了。
//
// secret 字段忽略 — 鉴权靠 URL 上的随机路径。

package webhook

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func init() {
	Register(teamsAdapter{})
}

type teamsAdapter struct{}

func (teamsAdapter) Name() string { return "teams" }

// Match 命中 Teams Incoming Webhook 新旧两套 URL 形态。
//
//	https://<tenant>.webhook.office.com/webhookb2/...   (现行)
//	https://outlook.office.com/webhook/...              (老入口)
func (teamsAdapter) Match(webhookURL string) bool {
	u := strings.ToLower(webhookURL)
	return strings.Contains(u, ".webhook.office.com/webhookb2/") ||
		strings.Contains(u, "outlook.office.com/webhook/")
}

// teamsMessageCard 见 https://learn.microsoft.com/en-us/outlook/actionable-messages/message-card-reference
type teamsMessageCard struct {
	Type       string `json:"@type"`
	Context    string `json:"@context"`
	Summary    string `json:"summary,omitempty"`
	ThemeColor string `json:"themeColor,omitempty"`
	Title      string `json:"title,omitempty"`
	Text       string `json:"text,omitempty"`
}

func (teamsAdapter) Send(webhookURL string, _ string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	title := strings.TrimSpace(data.Title)
	if title == "" {
		title = "Notification"
	}
	payload := teamsMessageCard{
		Type:    "MessageCard",
		Context: "http://schema.org/extensions",
		Summary: title,
		Title:   title,
		Text:    content,
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal teams payload: %v", err)
	}
	_, err = post(webhookURL, body, nil)
	return err
}
