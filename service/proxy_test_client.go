package service

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// TestProxyConnectivity 通过给定的代理 URL 访问 testTarget，判断代理是否可用。
// 返回 ok / 延迟毫秒 / 描述信息。
func TestProxyConnectivity(proxyURL, testTarget string, timeout time.Duration) (bool, int64, string) {
	if proxyURL == "" {
		return false, 0, "proxy url empty"
	}
	if testTarget == "" {
		return false, 0, "test target empty"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client, err := NewProxyHttpClient(proxyURL)
	if err != nil {
		return false, 0, fmt.Sprintf("build client: %v", err)
	}
	client.Timeout = timeout

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testTarget, nil)
	if err != nil {
		return false, 0, fmt.Sprintf("build request: %v", err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return false, latency, err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return false, latency, fmt.Sprintf("upstream %d", resp.StatusCode)
	}
	return true, latency, fmt.Sprintf("HTTP %d", resp.StatusCode)
}
