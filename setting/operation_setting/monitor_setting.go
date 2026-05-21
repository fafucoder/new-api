package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled    bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes    float64 `json:"auto_test_channel_minutes"`
	AutoBalanceAlertEnabled   bool    `json:"auto_balance_alert_enabled"`
	AutoBalanceAlertMinutes   float64 `json:"auto_balance_alert_minutes"`
	BalanceAlertCooldownMinutes float64 `json:"balance_alert_cooldown_minutes"`
}

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:    false,
	AutoTestChannelMinutes:    10,
	AutoBalanceAlertEnabled:   false,
	AutoBalanceAlertMinutes:   30,
	BalanceAlertCooldownMinutes: 60,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
		}
	}
	return &monitorSetting
}
