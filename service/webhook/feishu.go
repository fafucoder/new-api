// feishu.go: 飞书 / Lark 自定义机器人适配器。
//
// 通用 webhook 发的 {type,title,content,...} 飞书直接拒收 (HTTP 200 + code != 0,
// 不解 body 上层会误判成功 — "测试看着成功但机器人没收到"就是这个)。这里把
// 通知压成 msg_type=text 投递, 并把响应 body 里的 code 透成 error。
//
// 验签算法跟通用 webhook 不一样:
//
//	key = "<timestamp>\n<secret>"   // secret 同时进 key, msg 留空
//	sign = base64(HMAC-SHA256(key, ""))
//
// timestamp 和 sign 写到 payload 里 (不是 URL, 不是 header), 见
// https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/bot-v2/add-custom-bot

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func init() {
	Register(feishuAdapter{})
}

type feishuAdapter struct{}

func (feishuAdapter) Name() string { return "feishu" }

// Match 命中国内版 open.feishu.cn 和国际版 (Lark) open.larksuite.com,
// 路径都以 /open-apis/bot 开头。
func (feishuAdapter) Match(webhookURL string) bool {
	u := strings.ToLower(webhookURL)
	return strings.Contains(u, "open.feishu.cn/open-apis/bot") ||
		strings.Contains(u, "open.larksuite.com/open-apis/bot")
}

type feishuTextContent struct {
	Text string `json:"text"`
}

type feishuTextPayload struct {
	Timestamp string            `json:"timestamp,omitempty"`
	Sign      string            `json:"sign,omitempty"`
	MsgType   string            `json:"msg_type"`
	Content   feishuTextContent `json:"content"`
}

// feishuSign 计算飞书自定义机器人签名。
func feishuSign(secret string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (feishuAdapter) Send(webhookURL string, secret string, data dto.Notify) error {
	content := data.Content
	for _, value := range data.Values {
		content = fmt.Sprintf(content, value)
	}
	text := content
	if strings.TrimSpace(data.Title) != "" {
		text = data.Title + "\n\n" + content
	}
	payload := feishuTextPayload{
		MsgType: "text",
		Content: feishuTextContent{Text: text},
	}
	if secret != "" {
		ts := time.Now().Unix()
		payload.Timestamp = fmt.Sprintf("%d", ts)
		payload.Sign = feishuSign(secret, ts)
	}
	body, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal feishu payload: %v", err)
	}
	respBytes, err := post(webhookURL, body, nil)
	if err != nil {
		return err
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := common.Unmarshal(respBytes, &resp); err == nil && resp.Code != 0 {
		return fmt.Errorf("feishu rejected message: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}
