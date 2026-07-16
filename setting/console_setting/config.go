package console_setting

import "github.com/QuantumNous/new-api/setting/config"

type ConsoleSetting struct {
	ApiInfo              string `json:"api_info"`                // 控制台 API 信息 (JSON 数组字符串)
	UptimeKumaGroups     string `json:"uptime_kuma_groups"`      // Uptime Kuma 分组配置 (JSON 数组字符串)
	Announcements        string `json:"announcements"`           // 系统公告 (JSON 数组字符串)
	FAQ                  string `json:"faq"`                     // 常见问题 (JSON 数组字符串)
	ApiInfoEnabled       bool   `json:"api_info_enabled"`        // 是否启用 API 信息面板
	UptimeKumaEnabled    bool   `json:"uptime_kuma_enabled"`     // 是否启用 Uptime Kuma 面板
	AnnouncementsEnabled bool   `json:"announcements_enabled"`   // 是否启用系统公告面板
	FAQEnabled           bool   `json:"faq_enabled"`             // 是否启用常见问答面板
	CacheHitStatsEnabled bool   `json:"cache_hit_stats_enabled"` // 是否启用缓存命中率统计页面
	// LogDisplayUpstreamModelEnabled 控制普通用户使用日志是否展示「实际模型」(上游真实模型名)。
	// 默认 true（保持现状）；管理员始终可见，不受此开关影响。
	LogDisplayUpstreamModelEnabled bool `json:"log_display_upstream_model_enabled"`
}

// 默认配置
var defaultConsoleSetting = ConsoleSetting{
	ApiInfo:              "",
	UptimeKumaGroups:     "",
	Announcements:        "",
	FAQ:                  "",
	ApiInfoEnabled:       true,
	UptimeKumaEnabled:    true,
	AnnouncementsEnabled: true,
	FAQEnabled:           true,
	// 默认展示，保持升级前的现状；老部署 DB 无此键时也会沿用该默认值。
	LogDisplayUpstreamModelEnabled: true,
}

// 全局实例
var consoleSetting = defaultConsoleSetting

func init() {
	// 注册到全局配置管理器，键名为 console_setting
	config.GlobalConfig.Register("console_setting", &consoleSetting)
}

// GetConsoleSetting 获取 ConsoleSetting 配置实例
func GetConsoleSetting() *ConsoleSetting {
	return &consoleSetting
}
