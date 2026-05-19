// Package invoice — 业务编排: Apply/Issue/Reject/List/Summary.
//
// Apply 在事务里写 pending 记录, 同时算"可开票余额 = 充值成功总额
// - 已锁定开票总额", 校验是否达到最低开票额度, 并防止用户并发提交。
//
// Issue 用乐观锁推动 pending → issuing, 调用 provider, 成功后置
// issued, 失败回退 pending 并把错误写到 provider_raw。
//
// Reject 标记 pending 为 rejected。
package invoice

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var (
	ErrFeatureDisabled    = errors.New("invoice: feature disabled")
	ErrAmountBelowMinimum = errors.New("invoice: amount below minimum")
	ErrInFlightExists     = errors.New("invoice: in-flight invoice exists")
	ErrInvalidStatus      = errors.New("invoice: invalid status transition")
	ErrInvoiceNotFound    = errors.New("invoice: not found")
	ErrInvalidForm        = errors.New("invoice: invalid form")
)

// ApplyForm 是 controller 层透传给 service 的申请表单数据。
// 申请金额由 service 自己根据"可开票余额"算, 不接受前端传入,
// 防止客户端绕过校验。
type ApplyForm struct {
	ApplicantType string
	Title         string
	TaxID         string
	Email         string
	InvoiceType   string
}

// userApplyMu 简单的进程内锁, 防止同一用户两个请求并发拿到一致
// 的"可开票余额"快照后双发申请。多副本部署下需要 DB 行锁兜底,
// 但 v1 单副本场景这把锁已经足够。
var userApplyMu sync.Map // userID(int) -> *sync.Mutex

func lockUser(userID int) func() {
	v, _ := userApplyMu.LoadOrStore(userID, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// Apply 创建一条 pending 发票申请。若 RequireManualReview=false,
// 在事务提交后异步触发 Issue。
func Apply(userID int, form ApplyForm) (int, error) {
	setting := operation_setting.GetInvoiceSetting()
	if !setting.Enabled {
		return 0, ErrFeatureDisabled
	}
	if userID <= 0 {
		return 0, ErrInvalidForm
	}

	unlock := lockUser(userID)
	defer unlock()

	inFlight, err := model.HasInFlightInvoice(userID)
	if err != nil {
		return 0, err
	}
	if inFlight {
		return 0, ErrInFlightExists
	}

	topupSum, err := model.SumTopUpSuccessMoney(userID)
	if err != nil {
		return 0, err
	}
	invoicedSum, err := model.SumInvoicedAmount(userID)
	if err != nil {
		return 0, err
	}
	billable := topupSum - invoicedSum
	if billable < setting.MinimumAmount {
		return 0, ErrAmountBelowMinimum
	}

	inv := &model.Invoice{
		UserID:        userID,
		ApplicantType: form.ApplicantType,
		Title:         form.Title,
		TaxID:         form.TaxID,
		Email:         form.Email,
		InvoiceType:   form.InvoiceType,
		Amount:        billable,
		Status:        model.InvoiceStatusPending,
		Provider:      setting.Provider,
		AppliedAt:     common.GetTimestamp(),
	}
	if err := model.CreateInvoice(inv); err != nil {
		return 0, err
	}

	if !setting.RequireManualReview {
		go func(id int) {
			defer func() {
				if r := recover(); r != nil {
					common.SysError(fmt.Sprintf("invoice auto-issue panic: invoice=%d r=%v", id, r))
				}
			}()
			if err := Issue(id, 0); err != nil {
				common.SysError(fmt.Sprintf("invoice auto-issue failed: invoice=%d err=%v", id, err))
			}
		}(inv.Id)
	}
	return inv.Id, nil
}

// Issue 把 pending 发票通过 provider 出单。
// 状态机 pending → issuing(乐观锁) → issued / 回退 pending。
// reviewerID=0 表示自动出单(由 Apply 异步分支触发)。
func Issue(invoiceID int, reviewerID int) error {
	inv, err := model.GetInvoice(invoiceID)
	if err != nil {
		return err
	}
	if inv == nil {
		return ErrInvoiceNotFound
	}
	if inv.Status != model.InvoiceStatusPending {
		return ErrInvalidStatus
	}

	ok, err := model.TransitionInvoiceStatus(inv.Id,
		model.InvoiceStatusPending, model.InvoiceStatusIssuing, nil)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidStatus
	}

	// reload 拿到 issuing 后的最新 Provider 字段。极小概率重读失败
	// (比如 admin 同时把这条 invoice 删了), 防御一下 nil deref。
	reloaded, err := model.GetInvoice(invoiceID)
	if err != nil {
		rollback(invoiceID, fmt.Sprintf("reload failed: %s", err.Error()))
		return err
	}
	if reloaded == nil {
		rollback(invoiceID, "invoice gone after transition")
		return ErrInvoiceNotFound
	}
	inv = reloaded
	providerName := inv.Provider
	if providerName == "" {
		providerName = operation_setting.GetInvoiceSetting().Provider
	}

	p, err := Get(providerName)
	if err != nil {
		rollback(invoiceID, fmt.Sprintf("provider not registered: %s", err.Error()))
		return err
	}

	result, perr := safeIssue(p, &IssueRequest{
		InvoiceID:     inv.Id,
		UserID:        inv.UserID,
		ApplicantType: inv.ApplicantType,
		Title:         inv.Title,
		TaxID:         inv.TaxID,
		Email:         inv.Email,
		InvoiceType:   inv.InvoiceType,
		Amount:        inv.Amount,
	})
	if perr != nil {
		rollback(invoiceID, perr.Error())
		return perr
	}

	ok, err = model.TransitionInvoiceStatus(inv.Id,
		model.InvoiceStatusIssuing, model.InvoiceStatusIssued,
		map[string]any{
			"provider":            providerName,
			"provider_invoice_no": result.ProviderInvoiceNo,
			"provider_pdf_url":    result.PDFURL,
			"provider_raw":        result.RawResponse,
			"issued_at":           common.GetTimestamp(),
			"reviewer_id":         reviewerID,
		},
	)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInvalidStatus
	}

	if err := sendInvoiceIssuedEmail(inv, result); err != nil {
		common.SysError(fmt.Sprintf("invoice notify user failed: invoice=%d err=%v",
			inv.Id, err))
	}
	return nil
}

// safeIssue 包一层 recover, provider 实现 panic 时退化为 error。
func safeIssue(p InvoiceProvider, req *IssueRequest) (result *IssueResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("provider panic: %v", r)
		}
	}()
	return p.Issue(context.Background(), req)
}

// rollback 把 issuing 状态退回 pending, 错误写到 provider_raw。
// 用单条 UPDATE 完成, 不抛错(避免吞掉原始 issue 错误)。
func rollback(invoiceID int, reason string) {
	payload := map[string]any{
		"error": reason,
		"at":    common.GetTimestamp(),
	}
	rawBytes, err := common.Marshal(payload)
	if err != nil {
		// 极端情况:序列化失败回退到纯字符串, 不阻塞回滚动作
		rawBytes = []byte(fmt.Sprintf(`{"error":%q}`, reason))
	}
	_, err = model.TransitionInvoiceStatus(invoiceID,
		model.InvoiceStatusIssuing, model.InvoiceStatusPending,
		map[string]any{"provider_raw": string(rawBytes)},
	)
	if err != nil {
		common.SysError(fmt.Sprintf("invoice rollback failed: invoice=%d err=%v",
			invoiceID, err))
	}
}

// sendInvoiceIssuedEmail 邮件通知 — Task 8 实现, 这里先返回 nil。
func sendInvoiceIssuedEmail(inv *model.Invoice, result *IssueResult) error {
	return nil
}
