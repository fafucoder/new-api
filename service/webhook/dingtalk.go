// dingtalk.go: 钉钉自定义机器人适配器。
//
// schema: {msgtype:"text", text:{content:"..."}}, 错误码字段 errcode/errmsg
// (跟企微一样, 跟飞书的 code/msg 不一样)。
//
// 验签算法跟飞书像但又不一样:
//
//	key = secret
//	msg = "<timestamp_ms>\n<secret>"
//	sign = url_encode(base64(HMAC-SHA256(key, msg)))
//
// 注意是毫秒级 timestamp; 跟飞书 (秒级) 和通用 webhook 都不同。
// timestamp 和 sign 拼到 URL query 上, 不是 body — 这是跟飞书最大的格式差异。
// 见 https://open.dingtalk.com/document/robots/customize-robot-security-settings
//
// 钉钉机器人三种安全选项 (关键词 / IP 白名单 / 加签) 是机器人侧设的, 没配
// secret 时 content 必须含机器人配的关键词, 否则也会 errcode!=0 — 这是
// 配置问题, 错误消息会带原始 errmsg 透给用户排查。

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func init() {
	Register(dingTalkAdapter{})
}

type dingTalkAdapter struct{}

func (dingTalkAdapter) Name() string { return "dingtalk" }

// Match 命中 https://oapi.dingtalk.com/robot/send?access_token=...
func (dingTalkAdapter) Match(webhookURL string) bool {
	u := strings.ToLower(webhookURL)
	return strings.Contains(u, "oapi.dingtalk.com/robot/send")
}

type dingTalkText struct {
	Content string `json:"content"`
}

type dingTalkPayload struct {
	MsgType string       `json:"msgtype"`
	Text    dingTalkText `json:"text"`
}

func dingTalkSign(secret string, timestampMs int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestampMs, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
}

func (dingTalkAdapter) Send(webhookURL string, secret string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	text := content
	if strings.TrimSpace(data.Title) != "" {
		text = data.Title + "\n\n" + content
	}
	payload := dingTalkPayload{
		MsgType: "text",
		Text:    dingTalkText{Content: text},
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal dingtalk payload: %v", err)
	}

	finalURL := webhookURL
	if secret != "" {
		tsMs := time.Now().UnixMilli()
		sign := dingTalkSign(secret, tsMs)
		sep := "&"
		if !strings.Contains(finalURL, "?") {
			sep = "?"
		}
		finalURL = fmt.Sprintf("%s%stimestamp=%d&sign=%s", finalURL, sep, tsMs, sign)
	}

	respBytes, err := post(finalURL, body, nil)
	if err != nil {
		return err
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := common.Unmarshal(respBytes, &resp); err == nil && resp.ErrCode != 0 {
		return fmt.Errorf("dingtalk rejected message: errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}
