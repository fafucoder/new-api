package invoice

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestMain 初始化进程内 SQLite, 并把 service 测试依赖的表
// (Invoice + TopUp) AutoMigrate 出来。
// 与 model/task_cas_test.go 的 TestMain 思路一致, 但作用域是
// service/invoice 包: 不同的测试二进制需要各自的 TestMain 才能
// 拿到非 nil 的 model.DB。
func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db

	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true

	if err := db.AutoMigrate(
		&model.Invoice{},
		&model.TopUp{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

func setupInvoiceServiceDB(t *testing.T) {
	t.Helper()
	if model.DB == nil {
		t.Skip("DB not initialized")
	}
	// Invoice + TopUp migrated by TestMain.
	model.DB.Where("1=1").Delete(&model.Invoice{})
	model.DB.Where("1=1").Delete(&model.TopUp{})
}

func seedTopUp(t *testing.T, userID int, money float64, trade string) {
	t.Helper()
	tp := &model.TopUp{
		UserId: userID, Money: money, TradeNo: trade,
		Status: common.TopUpStatusSuccess,
	}
	if err := model.DB.Create(tp).Error; err != nil {
		t.Fatalf("seed topup: %v", err)
	}
}

func TestApply_DisabledFeature(t *testing.T) {
	setupInvoiceServiceDB(t)
	s := operation_setting.GetInvoiceSetting()
	prev := *s
	defer func() { *s = prev }()
	s.Enabled = false

	_, err := Apply(1, ApplyForm{
		ApplicantType: model.InvoiceApplicantPersonal,
		Title:         "x", Email: "a@b.com",
		InvoiceType: model.InvoiceTypeVATNormal,
	})
	if err != ErrFeatureDisabled {
		t.Fatalf("err = %v, want ErrFeatureDisabled", err)
	}
}

func TestApply_BelowMinimum(t *testing.T) {
	setupInvoiceServiceDB(t)
	s := operation_setting.GetInvoiceSetting()
	prev := *s
	defer func() { *s = prev }()
	s.Enabled = true
	s.MinimumAmount = 100
	s.RequireManualReview = true

	seedTopUp(t, 1, 50, "tinv-below-1")

	_, err := Apply(1, ApplyForm{
		ApplicantType: model.InvoiceApplicantPersonal,
		Title:         "x", Email: "a@b.com",
		InvoiceType: model.InvoiceTypeVATNormal,
	})
	if err != ErrAmountBelowMinimum {
		t.Fatalf("err = %v, want ErrAmountBelowMinimum", err)
	}
}

func TestApply_InFlightExists(t *testing.T) {
	setupInvoiceServiceDB(t)
	s := operation_setting.GetInvoiceSetting()
	prev := *s
	defer func() { *s = prev }()
	s.Enabled = true
	s.MinimumAmount = 1
	s.RequireManualReview = true

	seedTopUp(t, 1, 1000, "tinv-inflight-1")
	model.DB.Create(&model.Invoice{
		UserID: 1, ApplicantType: model.InvoiceApplicantPersonal,
		Title: "x", Email: "a@b.com", InvoiceType: model.InvoiceTypeVATNormal,
		Amount: 100, Status: model.InvoiceStatusPending,
		AppliedAt: common.GetTimestamp(),
	})

	_, err := Apply(1, ApplyForm{
		ApplicantType: model.InvoiceApplicantPersonal,
		Title:         "x", Email: "a@b.com",
		InvoiceType: model.InvoiceTypeVATNormal,
	})
	if err != ErrInFlightExists {
		t.Fatalf("err = %v, want ErrInFlightExists", err)
	}
}

func TestApply_OK_ManualReview(t *testing.T) {
	setupInvoiceServiceDB(t)
	s := operation_setting.GetInvoiceSetting()
	prev := *s
	defer func() { *s = prev }()
	s.Enabled = true
	s.MinimumAmount = 50
	s.RequireManualReview = true
	s.Provider = "stub"

	seedTopUp(t, 1, 300, "tinv-ok-1")

	id, err := Apply(1, ApplyForm{
		ApplicantType: model.InvoiceApplicantPersonal,
		Title:         "Alice", Email: "a@b.com",
		InvoiceType: model.InvoiceTypeVATNormal,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	inv, _ := model.GetInvoice(id)
	if inv == nil || inv.Status != model.InvoiceStatusPending {
		t.Fatalf("inv = %+v", inv)
	}
	if inv.Amount != 300 {
		t.Fatalf("amount = %v, want 300 (full billable)", inv.Amount)
	}
}

type fakeProvider struct {
	name string
	fn   func(req *IssueRequest) (*IssueResult, error)
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Issue(ctx context.Context, req *IssueRequest) (*IssueResult, error) {
	return f.fn(req)
}

func registerFakeProvider(t *testing.T, name string, fn func(*IssueRequest) (*IssueResult, error)) {
	t.Helper()
	Register(name, func() InvoiceProvider { return &fakeProvider{name: name, fn: fn} })
}

func seedPendingInvoice(t *testing.T, userID int, amount float64, provider string) *model.Invoice {
	t.Helper()
	inv := &model.Invoice{
		UserID: userID, ApplicantType: model.InvoiceApplicantPersonal,
		Title: "x", Email: "a@b.com", InvoiceType: model.InvoiceTypeVATNormal,
		Amount: amount, Status: model.InvoiceStatusPending,
		Provider:  provider,
		AppliedAt: common.GetTimestamp(),
	}
	if err := model.DB.Create(inv).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	return inv
}

func TestIssue_NotFound(t *testing.T) {
	setupInvoiceServiceDB(t)
	if err := Issue(99999, 1); err != ErrInvoiceNotFound {
		t.Fatalf("err = %v, want ErrInvoiceNotFound", err)
	}
}

func TestIssue_WrongStatus(t *testing.T) {
	setupInvoiceServiceDB(t)
	inv := seedPendingInvoice(t, 1, 100, "stub")
	model.DB.Model(&model.Invoice{}).Where("id=?", inv.Id).
		Update("status", model.InvoiceStatusIssued)

	if err := Issue(inv.Id, 1); err != ErrInvalidStatus {
		t.Fatalf("err = %v, want ErrInvalidStatus", err)
	}
}

func TestIssue_ProviderSuccess(t *testing.T) {
	setupInvoiceServiceDB(t)
	registerFakeProvider(t, "fake-ok", func(req *IssueRequest) (*IssueResult, error) {
		return &IssueResult{
			ProviderInvoiceNo: "FAKE-1",
			PDFURL:            "https://example.com/x.pdf",
			RawResponse:       `{"ok":true}`,
		}, nil
	})
	inv := seedPendingInvoice(t, 1, 100, "fake-ok")

	if err := Issue(inv.Id, 7); err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, _ := model.GetInvoice(inv.Id)
	if got.Status != model.InvoiceStatusIssued {
		t.Fatalf("status = %q, want issued", got.Status)
	}
	if got.ProviderInvoiceNo != "FAKE-1" {
		t.Fatalf("no = %q", got.ProviderInvoiceNo)
	}
	if got.ProviderPDFURL != "https://example.com/x.pdf" {
		t.Fatalf("pdf url not saved")
	}
	if got.ReviewerID != 7 {
		t.Fatalf("reviewer = %v", got.ReviewerID)
	}
	if got.IssuedAt == 0 {
		t.Fatalf("issued_at not set")
	}
}

func TestIssue_ProviderFailureRollsBack(t *testing.T) {
	setupInvoiceServiceDB(t)
	registerFakeProvider(t, "fake-fail", func(req *IssueRequest) (*IssueResult, error) {
		return nil, errors.New("upstream 500")
	})
	inv := seedPendingInvoice(t, 1, 100, "fake-fail")

	if err := Issue(inv.Id, 0); err == nil {
		t.Fatal("expected error from provider")
	}
	got, _ := model.GetInvoice(inv.Id)
	if got.Status != model.InvoiceStatusPending {
		t.Fatalf("status = %q, want pending (rollback)", got.Status)
	}
	if !strings.Contains(got.ProviderRaw, "upstream 500") {
		t.Fatalf("error not recorded in provider_raw: %q", got.ProviderRaw)
	}
}
