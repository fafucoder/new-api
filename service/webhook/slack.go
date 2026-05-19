// slack.go: Slack Incoming Webhook 适配器。
//
// schema 最简版 {text:"..."}, 支持 markdown。Slack 不像飞书那样把错误码塞
// 在 200 body 里 — 它要么返 200 + 文本 "ok" 表示成功, 要么返 4xx + 错误
// 文本 (如 "invalid_payload", "no_text"), HTTP 状态码就够判断结果。
// 但为了把具体原因带回上层, 还是检查一下 body 是否真的是 "ok"。
//
// secret 字段在 Slack 里没用 — 鉴权完全靠 URL 里的 token 路径,
// 所以这里显式忽略。
//
// 文档: https://api.slack.com/messaging/webhooks

package webhook

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func init() {
	Register(slackAdapter{})
}

type slackAdapter struct{}

func (slackAdapter) Name() string { return "slack" }

// Match 命中 https://hooks.slack.com/services/T.../B.../...
func (slackAdapter) Match(webhookURL string) bool {
	u := strings.ToLower(webhookURL)
	return strings.Contains(u, "hooks.slack.com/services/")
}

type slackPayload struct {
	Text string `json:"text"`
}

func (slackAdapter) Send(webhookURL string, _ string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	text := content
	if strings.TrimSpace(data.Title) != "" {
		text = "*" + data.Title + "*\n\n" + content
	}
	payload := slackPayload{Text: text}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %v", err)
	}
	respBytes, err := post(webhookURL, body, nil)
	if err != nil {
		return err
	}
	bodyStr := strings.TrimSpace(string(respBytes))
	if bodyStr != "" && bodyStr != "ok" {
		return fmt.Errorf("slack rejected message: %s", bodyStr)
	}
	return nil
}
