package dto

// InvoiceApplyRequest 用户提交开票申请的请求体。
// 申请金额由后端根据"可开票余额"算, 不接受前端传入。
type InvoiceApplyRequest struct {
	ApplicantType string `json:"applicant_type" binding:"required,oneof=personal enterprise"`
	Title         string `json:"title" binding:"required,max=128"`
	TaxID         string `json:"tax_id" binding:"max=32"`
	Email         string `json:"email" binding:"required,email,max=128"`
	InvoiceType   string `json:"invoice_type" binding:"required,oneof=vat_normal vat_special"`
}

// InvoiceRejectRequest 管理员拒绝开票请求的请求体。
type InvoiceRejectRequest struct {
	Reason string `json:"reason" binding:"max=256"`
}
