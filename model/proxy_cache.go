package model

import (
	"errors"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var (
	proxyCache   map[int]*Proxy
	proxyCacheMu sync.RWMutex

	ErrProxyNotFound = errors.New("proxy_not_found")
	ErrProxyDisabled = errors.New("proxy_disabled")
)

func InitProxyCache() {
	if !common.MemoryCacheEnabled {
		return
	}
	newMap := make(map[int]*Proxy)
	var proxies []*Proxy
	if err := DB.Find(&proxies).Error; err != nil {
		common.SysLog("InitProxyCache: failed to load proxies: " + err.Error())
		return
	}
	for _, p := range proxies {
		newMap[p.Id] = p
	}
	proxyCacheMu.Lock()
	proxyCache = newMap
	proxyCacheMu.Unlock()
}

func InvalidateProxyCache() {
	InitProxyCache()
}

func GetCachedProxy(id int) (*Proxy, bool) {
	proxyCacheMu.RLock()
	defer proxyCacheMu.RUnlock()
	if proxyCache == nil {
		return nil, false
	}
	p, ok := proxyCache[id]
	return p, ok
}

// ResolveChannelProxy 根据 channel 的 ProxyId 返回代理 URL。
// - proxyId nil 或 0：返回空字符串（直连），无错误。
// - 代理不存在：ErrProxyNotFound。
// - 代理禁用：ErrProxyDisabled。
func ResolveChannelProxy(proxyId *int) (string, error) {
	if proxyId == nil || *proxyId <= 0 {
		return "", nil
	}
	id := *proxyId
	if common.MemoryCacheEnabled {
		if p, ok := GetCachedProxy(id); ok {
			if p.Status != ProxyStatusEnabled {
				return "", ErrProxyDisabled
			}
			return p.URL, nil
		}
		return "", ErrProxyNotFound
	}
	p, err := GetProxyById(id)
	if err != nil {
		return "", ErrProxyNotFound
	}
	if p.Status != ProxyStatusEnabled {
		return "", ErrProxyDisabled
	}
	return p.URL, nil
}
