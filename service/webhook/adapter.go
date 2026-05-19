// adapter.go: webhook 适配器接口 + 注册表。
//
// 加新平台只需做两件事:
//
//  1. 新建 <platform>.go, 写一个 struct 实现 Adapter (Name/Match/Send);
//  2. 在该文件 init() 里调一次 Register(<adapter>{})。
//
// Send() 自动会用上, 不用改任何 dispatch 代码 — 这就是用注册表代替 switch
// 的目的。约定: 各家 Match 只做廉价的 URL 形态判定 (不发网络/不读 DB);
// Send 必须把"HTTP 200 但业务码 != 0"这类沉默错误透成 error, 否则 caller
// 会误判为投递成功。
//
// 注册时机: 各文件 init() 在包 import 时被 runtime 调一次, 单线程, 顺序
// 是文件名字典序 — 但我们这套适配器 URL 模式互不重叠, 顺序不影响命中。
// 用 sync.RWMutex 留作 future-proof, 后续就算运行时 Register 也安全。

package webhook

import (
	"sync"

	"github.com/QuantumNous/new-api/dto"
)

// Adapter 是 webhook 适配器的统一形状。
type Adapter interface {
	// Name 仅给日志/错误信息用, 不参与 dispatch。
	Name() string
	// Match 仅做 URL 形态判定, 用 strings.Contains / url.Parse 之类廉价检查。
	Match(webhookURL string) bool
	// Send 真正投递。约定: 平台业务层错误码 (飞书 code, 钉钉/企微 errcode,
	// telegram ok=false 等) 必须解析后透成 error, 不要光看 HTTP 200。
	Send(webhookURL string, secret string, data dto.Notify) error
}

var (
	adaptersMu sync.RWMutex
	adapters   []Adapter
)

// Register 把适配器加进 dispatch 表。各平台文件在自己的 init() 里调一次。
func Register(a Adapter) {
	adaptersMu.Lock()
	defer adaptersMu.Unlock()
	adapters = append(adapters, a)
}

// pickAdapter 返回首个匹配 URL 的适配器, 没找到返 nil (由 Send 兜底走
// generic)。多个适配器同时 Match 的情况目前不存在 — 各平台 host 互斥,
// 真要扩展时再加 Priority。
func pickAdapter(webhookURL string) Adapter {
	adaptersMu.RLock()
	defer adaptersMu.RUnlock()
	for _, a := range adapters {
		if a.Match(webhookURL) {
			return a
		}
	}
	return nil
}
