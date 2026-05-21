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
	InvoiceTopupSourceTopUps = "top_ups"
	InvoiceTopupSourceUsers  = "users"
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
	TopupSource       string  `json:"topup_source" gorm:"type:varchar(16);default:'top_ups'"`
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

// SumInvoicedAmount 算指定用户的"已锁定开票金额":
// pending + issuing + issued 都计入, rejected 不算。
// topupSource 用于只统计同来源的发票，避免跨模式重复计算。
func SumInvoicedAmount(userID int, topupSource string) (float64, error) {
	if userID <= 0 {
		return 0, errors.New("invalid user id")
	}
	var sum float64
	err := DB.Model(&Invoice{}).
		Where("user_id = ? AND topup_source = ? AND status IN ?", userID, topupSource, []string{
			InvoiceStatusPending, InvoiceStatusIssuing, InvoiceStatusIssued,
		}).
		Select("COALESCE(SUM(amount), 0)").Scan(&sum).Error
	return sum, err
}

// HasInFlightInvoice 用户是否还有处理中的发票申请(pending / issuing)。
// 用来防止同一用户并发发起多条申请。
func HasInFlightInvoice(userID int) (bool, error) {
	if userID <= 0 {
		return false, errors.New("invalid user id")
	}
	var cnt int64
	err := DB.Model(&Invoice{}).
		Where("user_id = ? AND status IN ?", userID, []string{
			InvoiceStatusPending, InvoiceStatusIssuing,
		}).
		Count(&cnt).Error
	return cnt > 0, err
}

// TransitionInvoiceStatus 用乐观锁推进状态:
// UPDATE invoices SET status=? [, extra...] WHERE id=? AND status=?
// 返回 (true, nil) 表示成功;(false, nil) 表示状态已变更;
// (_, err) 表示底层错误。
//
// extra 用来在同一条 UPDATE 里写入回执字段(provider_invoice_no 等),
// 减少 race window。传 nil 表示只改 status。
func TransitionInvoiceStatus(id int, from, to string, extra map[string]any) (bool, error) {
	if id <= 0 {
		return false, errors.New("invalid invoice id")
	}
	updates := map[string]any{"status": to}
	for k, v := range extra {
		updates[k] = v
	}
	res := DB.Model(&Invoice{}).
		Where("id = ? AND status = ?", id, from).
		Updates(updates)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// SumUserTotalQuota 从 users 表计算用户已消费额度并换算为 USD。
// USD = used_quota / QuotaPerUnit
func SumUserTotalQuota(userID int) (float64, error) {
	if userID <= 0 {
		return 0, errors.New("invalid user id")
	}
	var user User
	if err := DB.Select("used_quota").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return float64(user.UsedQuota) / common.QuotaPerUnit, nil
}

// SumTopUpSuccessMoney 用户成功充值的总金额(USD)。
// 与 SumInvoicedAmount 配合得出"可开票余额"。
// 注意:money 字段是充值金额(USD), 不要和 amount(配额单位) 混。
func SumTopUpSuccessMoney(userID int) (float64, error) {
	if userID <= 0 {
		return 0, errors.New("invalid user id")
	}
	var sum float64
	err := DB.Model(&TopUp{}).
		Where("user_id = ? AND status = ?", userID, common.TopUpStatusSuccess).
		Select("COALESCE(SUM(money), 0)").Scan(&sum).Error
	return sum, err
}

// ListInvoicesForUser 拿用户自己的发票列表, 按申请时间倒序。
func ListInvoicesForUser(userID int, page, pageSize int) ([]Invoice, int64, error) {
	if userID <= 0 {
		return nil, 0, errors.New("invalid user id")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	var total int64
	if err := DB.Model(&Invoice{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Invoice
	err := DB.Where("user_id = ?", userID).
		Order("applied_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}

// InvoiceListFilter admin 查询过滤条件。零值字段被跳过。
type InvoiceListFilter struct {
	Statuses    []string
	UserID      int
	AppliedFrom int64
	AppliedTo   int64
}

// AdminListInvoices 管理员侧列表, 支持状态 / 用户 / 时间区间筛选。
func AdminListInvoices(filter InvoiceListFilter, page, pageSize int) ([]Invoice, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	q := DB.Model(&Invoice{})
	if len(filter.Statuses) > 0 {
		q = q.Where("status IN ?", filter.Statuses)
	}
	if filter.UserID > 0 {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.AppliedFrom > 0 {
		q = q.Where("applied_at >= ?", filter.AppliedFrom)
	}
	if filter.AppliedTo > 0 {
		q = q.Where("applied_at <= ?", filter.AppliedTo)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []Invoice
	err := q.Order("applied_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error
	return items, total, err
}
