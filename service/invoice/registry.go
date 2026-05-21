package invoice

import (
	"errors"
	"sync"
)

var (
	providerMu      sync.RWMutex
	providerFactory = map[string]func() InvoiceProvider{}
)

// Register 把一个 provider 工厂登记到全局表里。在 init() 时调用,
// 同 key 重复注册会被后者覆盖(测试场景方便注入 mock)。
func Register(name string, factory func() InvoiceProvider) {
	providerMu.Lock()
	defer providerMu.Unlock()
	providerFactory[name] = factory
}

// Get 按名称取一个新的 provider 实例。返回新实例而非单例, 让
// 实现可以持有请求级别的本地状态(retry 计数等)。
func Get(name string) (InvoiceProvider, error) {
	providerMu.RLock()
	defer providerMu.RUnlock()
	factory, ok := providerFactory[name]
	if !ok {
		return nil, errors.New("invoice provider not registered: " + name)
	}
	return factory(), nil
}

func init() {
	Register("stub", func() InvoiceProvider { return &stubProvider{} })
}
