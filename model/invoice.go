// Package model — invoice: 发票申请记录。
//
// 单表存全部字段, 包含申请快照(抬头/税号/邮箱/类型/金额)和第三方
// 开票回执(provider/provider_invoice_no/provider_pdf_url/raw)。
// 状态机 pending → issuing → issued / rejected, 状态翻转由 service 层
// 用乐观锁推进。
package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	InvoiceApplicantPersonal   = "personal"
	InvoiceApplicantEnterprise = "enterprise"

	InvoiceTypeVATNormal  = "vat_normal"
	InvoiceTypeVATSpecial = "vat_special"

	InvoiceStatusPending  = "pending"
	InvoiceStatusIssuing  = "issuing"
	InvoiceStatusIssued   = "issued"
	InvoiceStatusRejected = "rejected"
)

type Invoice struct {
	Id                int     `json:"id" gorm:"primaryKey"`
	UserID            int     `json:"user_id" gorm:"index"`
	ApplicantType     string  `json:"applicant_type" gorm:"type:varchar(16)"`
	Title             string  `json:"title" gorm:"type:varchar(128)"`
	TaxID             string  `json:"tax_id" gorm:"type:varchar(32)"`
	Email             string  `json:"email" gorm:"type:varchar(128)"`
	InvoiceType       string  `json:"invoice_type" gorm:"type:varchar(16)"`
	Amount            float64 `json:"amount" gorm:"type:decimal(16,4)"`
	Status            string  `json:"status" gorm:"type:varchar(16);index;default:'pending'"`
	RejectReason      string  `json:"reject_reason" gorm:"type:varchar(256)"`
	ReviewerID        int     `json:"reviewer_id"`
	Provider          string  `json:"provider" gorm:"type:varchar(32)"`
	ProviderInvoiceNo string  `json:"provider_invoice_no" gorm:"type:varchar(64)"`
	ProviderPDFURL    string  `json:"provider_pdf_url" gorm:"type:varchar(512)"`
	ProviderRaw       string  `json:"provider_raw" gorm:"type:text"`
	AppliedAt         int64   `json:"applied_at"`
	IssuedAt          int64   `json:"issued_at"`
	CreatedTime       int64   `json:"created_time" gorm:"bigint;autoCreateTime"`
	UpdatedTime       int64   `json:"updated_time" gorm:"bigint;autoUpdateTime"`
}

func (Invoice) TableName() string { return "invoices" }

func validateInvoiceFields(inv *Invoice) error {
	if inv.UserID <= 0 {
		return errors.New("user_id is required")
	}
	inv.Title = strings.TrimSpace(inv.Title)
	if inv.Title == "" {
		return errors.New("title is required")
	}
	inv.Email = strings.TrimSpace(inv.Email)
	if inv.Email == "" {
		return errors.New("email is required")
	}
	inv.TaxID = strings.TrimSpace(inv.TaxID)
	switch inv.ApplicantType {
	case InvoiceApplicantPersonal:
		if inv.TaxID != "" {
			return errors.New("personal applicant must not have tax_id")
		}
		if inv.InvoiceType != InvoiceTypeVATNormal {
			return errors.New("personal applicant must use vat_normal")
		}
	case InvoiceApplicantEnterprise:
		if inv.TaxID == "" {
			return errors.New("enterprise applicant requires tax_id")
		}
		if inv.InvoiceType != InvoiceTypeVATNormal && inv.InvoiceType != InvoiceTypeVATSpecial {
			return errors.New("invalid invoice_type")
		}
	default:
		return errors.New("invalid applicant_type")
	}
	if inv.Amount <= 0 {
		return errors.New("amount must be > 0")
	}
	return nil
}

// CreateInvoice 写入新申请。调用方应已经做了业务校验
// (可开票余额检查、in-flight 检测), 这里只做字段级校验
// 防止脏数据落库。
func CreateInvoice(inv *Invoice) error {
	if inv == nil {
		return errors.New("nil invoice")
	}
	if err := validateInvoiceFields(inv); err != nil {
		return err
	}
	if inv.Status == "" {
		inv.Status = InvoiceStatusPending
	}
	if inv.AppliedAt == 0 {
		inv.AppliedAt = common.GetTimestamp()
	}
	return DB.Create(inv).Error
}

// GetInvoice 加载单条, 找不到返回 (nil, nil)。
func GetInvoice(id int) (*Invoice, error) {
	if id <= 0 {
		return nil, errors.New("invalid invoice id")
	}
	var inv Invoice
	if err := DB.First(&inv, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &inv, nil
}
