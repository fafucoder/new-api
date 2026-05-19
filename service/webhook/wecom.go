// wecom.go: 企业微信群机器人适配器。
//
// schema 跟钉钉一样是 {msgtype:"text", text:{content:"..."}}, 错误码也是
// errcode/errmsg。最大差别是不需要签名: 鉴权完全靠 URL 上的 ?key=... 参数,
// 所以这里把传进来的 secret 显式忽略 (保留参数是为了跟其它适配器同签名)。
//
// 文档: https://developer.work.weixin.qq.com/document/path/91770

package webhook

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func init() {
	Register(weComAdapter{})
}

type weComAdapter struct{}

func (weComAdapter) Name() string { return "wecom" }

// Match 命中 https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...
func (weComAdapter) Match(webhookURL string) bool {
	u := strings.ToLower(webhookURL)
	return strings.Contains(u, "qyapi.weixin.qq.com/cgi-bin/webhook/send")
}

type weComText struct {
	Content string `json:"content"`
}

type weComPayload struct {
	MsgType string    `json:"msgtype"`
	Text    weComText `json:"text"`
}

func (weComAdapter) Send(webhookURL string, _ string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	text := content
	if strings.TrimSpace(data.Title) != "" {
		text = data.Title + "\n\n" + content
	}
	payload := weComPayload{
		MsgType: "text",
		Text:    weComText{Content: text},
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal wecom payload: %v", err)
	}
	respBytes, err := post(webhookURL, body, nil)
	if err != nil {
		return err
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := common.Unmarshal(respBytes, &resp); err == nil && resp.ErrCode != 0 {
		return fmt.Errorf("wecom rejected message: errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}
