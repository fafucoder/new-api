// discord.go: Discord Webhook 适配器。
//
// schema 最简版 {content:"..."}, 支持 markdown。content 上限 2000 字, 超了
// 就截一下不让整条 POST 因为长度被拒。
//
// Discord 成功是 204 No Content (body 空), post() 已经处理过非 2xx → error,
// 所以这里不需要再解 body — 拿到 nil 就是成功。
//
// secret 字段忽略 — Discord webhook 的鉴权完全靠 URL 路径里的 token。
//
// 文档: https://discord.com/developers/docs/resources/webhook#execute-webhook

package webhook

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func init() {
	Register(discordAdapter{})
}

type discordAdapter struct{}

func (discordAdapter) Name() string { return "discord" }

// Match 命中新旧两个 Discord webhook 域名。
//
//	https://discord.com/api/webhooks/<id>/<token>
//	https://discordapp.com/api/webhooks/<id>/<token>   (老域名)
func (discordAdapter) Match(webhookURL string) bool {
	u := strings.ToLower(webhookURL)
	return strings.Contains(u, "discord.com/api/webhooks/") ||
		strings.Contains(u, "discordapp.com/api/webhooks/")
}

type discordPayload struct {
	Content string `json:"content"`
}

func (discordAdapter) Send(webhookURL string, _ string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	text := content
	if strings.TrimSpace(data.Title) != "" {
		text = "**" + data.Title + "**\n\n" + content
	}
	// Discord content 上限 2000, 超了整条会被拒。
	const maxLen = 2000
	if len(text) > maxLen {
		text = text[:maxLen-3] + "..."
	}
	payload := discordPayload{Content: text}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %v", err)
	}
	_, err = post(webhookURL, body, nil)
	return err
}
