package invoice

import (
	"os"
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
