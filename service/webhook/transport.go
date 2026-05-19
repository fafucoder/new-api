// transport.go: webhook 包共用的 HTTP 收发底座。
//
// 各适配器只负责组装 body / 签名 / 解析响应。SSRF 校验、Worker 模式分支、
// 状态码检查这套传输层逻辑统一收在这, 各家适配器调 post() 拿 response
// body 就行。
//
// Worker 模式: 项目里 service.DoWorkerRequest 已经做了同样的事, 但它在
// service 包里 — 反过来 import 会跟 service/balance_alert.go 等形成循环。
// 所以这里把 worker 信封 (URL + Key + Headers + Body) 自己复刻一遍,
// schema 跟 service.WorkerRequest 保持一致。

package webhook

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// workerEnvelope 跟 service.WorkerRequest 同 schema, Cloudflare Worker 那
// 边只认这个字段集。Body 用 []byte (而非 json.RawMessage) 是因为我们走
// common.Marshal 统一序列化, 上游适配器交进来时已经是 JSON bytes。
type workerEnvelope struct {
	URL     string            `json:"url"`
	Key     string            `json:"key"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

var (
	clientOnce sync.Once
	client     *http.Client
)

// httpClient 返回包内共用的 client。默认 30s 超时 + redirect SSRF 校验
// (跟 service.GetHttpClient() 一致), 调用方按需 SetHttpClient 覆盖。
func httpClient() *http.Client {
	clientOnce.Do(func() {
		if client == nil {
			client = &http.Client{
				Timeout:       30 * time.Second,
				CheckRedirect: checkRedirect,
			}
		}
	})
	return client
}

// SetHttpClient 让 host 进程注入项目自带的 client (带项目超时/代理/TLS
// 设置)。在 init 阶段调一次即可; 不调也能跑, 走默认。
func SetHttpClient(c *http.Client) {
	client = c
}

// checkRedirect 在重定向跟随时把每跳目标过一遍 SSRF 校验, 防止 webhook
// 接收方把我们重定向到内网服务。
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	fs := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(req.URL.String(), fs.EnableSSRFProtection, fs.AllowPrivateIp, fs.DomainFilterMode, fs.IpFilterMode, fs.DomainList, fs.IpList, fs.AllowedPorts, fs.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", req.URL.String(), err)
	}
	return nil
}

// post 把 body POST 到 webhookURL。Worker 模式启用时改走 worker 转发,
// 否则直连。返回响应 body 让上层按各家协议解业务码 (飞书 code, 钉钉/企微
// errcode); HTTP 非 2xx 时也尽量带上响应内容片段, 便于排查。
func post(webhookURL string, body []byte, extraHeaders map[string]string) ([]byte, error) {
	if system_setting.EnableWorker() {
		return postViaWorker(webhookURL, body, extraHeaders)
	}
	fs := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(webhookURL, fs.EnableSSRFProtection, fs.AllowPrivateIp, fs.DomainFilterMode, fs.IpFilterMode, fs.DomainList, fs.IpList, fs.AllowedPorts, fs.ApplyIPFilterForDomain); err != nil {
		return nil, fmt.Errorf("request reject: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create webhook request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("send webhook request: %v", err)
	}
	defer resp.Body.Close()
	return readResponse(resp)
}

func postViaWorker(targetURL string, body []byte, extraHeaders map[string]string) ([]byte, error) {
	if !system_setting.WorkerAllowHttpImageRequestEnabled && !strings.HasPrefix(targetURL, "https") {
		return nil, fmt.Errorf("worker only supports https url")
	}
	fs := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(targetURL, fs.EnableSSRFProtection, fs.AllowPrivateIp, fs.DomainFilterMode, fs.IpFilterMode, fs.DomainList, fs.IpList, fs.AllowedPorts, fs.ApplyIPFilterForDomain); err != nil {
		return nil, fmt.Errorf("request reject: %v", err)
	}
	headers := map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	envelope := workerEnvelope{
		URL:     targetURL,
		Key:     system_setting.WorkerValidKey,
		Method:  http.MethodPost,
		Headers: headers,
		Body:    body,
	}
	payload, err := common.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal worker envelope: %v", err)
	}
	workerURL := system_setting.WorkerUrl
	if !strings.HasSuffix(workerURL, "/") {
		workerURL += "/"
	}
	resp, err := httpClient().Post(workerURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("send webhook through worker: %v", err)
	}
	defer resp.Body.Close()
	return readResponse(resp)
}

func readResponse(resp *http.Response) ([]byte, error) {
	body, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := ""
		if readErr == nil && len(body) > 0 {
			const max = 200
			s := string(body)
			if len(s) > max {
				s = s[:max] + "..."
			}
			snippet = ": " + s
		}
		return body, fmt.Errorf("webhook http status %d%s", resp.StatusCode, snippet)
	}
	if readErr != nil {
		return nil, fmt.Errorf("read webhook response: %v", readErr)
	}
	return body, nil
}
