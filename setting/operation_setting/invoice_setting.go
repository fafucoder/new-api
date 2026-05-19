package operation_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

type InvoiceSetting struct {
	Enabled             bool    `json:"enabled"`
	MinimumAmount       float64 `json:"minimum_amount"`
	RequireManualReview bool    `json:"require_manual_review"`
	Provider            string  `json:"provider"`
}

// 默认配置
var invoiceSetting = InvoiceSetting{
	Enabled:             false,
	MinimumAmount:       50.0,
	RequireManualReview: true,
	Provider:            "stub",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("invoice_setting", &invoiceSetting)
}

func GetInvoiceSetting() *InvoiceSetting {
	return &invoiceSetting
}
