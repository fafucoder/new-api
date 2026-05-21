// Package invoice — 发票申请业务编排 + 第三方开票 provider 抽象。
//
// Provider 接口让业务代码与具体开票服务商解耦, v1 仅提供 stub。
// 后续接讯汇/腾讯电子发票/百望等只需要新增一个 provider 实现文件
// 并在 init() 里 Register。
package invoice

import "context"

// IssueRequest 是调用第三方开票 API 所需的最小信息。
// Provider 实现可以从 Title/TaxID/Email/InvoiceType/Amount 拼出
// 自己的请求体, InvoiceID 用于回写日志关联本地记录。
type IssueRequest struct {
	InvoiceID     int
	UserID        int
	ApplicantType string  // personal | enterprise
	Title         string
	TaxID         string  // 个人为空, 企业必填
	Email         string
	InvoiceType   string  // vat_normal | vat_special
	Amount        float64 // USD
}

// IssueResult 是开票成功后写回 invoices 表的字段。
// RawResponse 是 provider 的原始响应 JSON, 留作排查证据。
type IssueResult struct {
	ProviderInvoiceNo string
	PDFURL            string
	RawResponse       string
}

type InvoiceProvider interface {
	Name() string
	Issue(ctx context.Context, req *IssueRequest) (*IssueResult, error)
}
