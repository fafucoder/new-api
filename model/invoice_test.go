package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// setupInvoiceTestDB 依赖 TestMain (model/task_cas_test.go) 把 Invoice 表
// 一并 AutoMigrate 好;这里只清空表, 避免在 SQLite 上 ALTER COLUMN
// 引发的 "invalid DDL" 错误。
func setupInvoiceTestDB(t *testing.T) {
	t.Helper()
	if DB == nil {
		t.Skip("DB not initialized; integration test")
	}
	DB.Where("1=1").Delete(&Invoice{})
}

func TestCreateInvoice_ValidatesRequired(t *testing.T) {
	setupInvoiceTestDB(t)

	cases := []struct {
		name    string
		inv     *Invoice
		wantErr string
	}{
		{"nil", nil, "nil invoice"},
		{"missing user", &Invoice{ApplicantType: InvoiceApplicantPersonal, Title: "x", Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal, Amount: 100}, "user_id is required"},
		{"missing title", &Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal, Amount: 100}, "title is required"},
		{"missing email", &Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Title: "x", InvoiceType: InvoiceTypeVATNormal, Amount: 100}, "email is required"},
		{"personal + tax_id", &Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Title: "x", Email: "a@b.com", TaxID: "T1", InvoiceType: InvoiceTypeVATNormal, Amount: 100}, "personal applicant must not have tax_id"},
		{"personal + special", &Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Title: "x", Email: "a@b.com", InvoiceType: InvoiceTypeVATSpecial, Amount: 100}, "personal applicant must use vat_normal"},
		{"enterprise no tax_id", &Invoice{UserID: 1, ApplicantType: InvoiceApplicantEnterprise, Title: "x", Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal, Amount: 100}, "enterprise applicant requires tax_id"},
		{"amount zero", &Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Title: "x", Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal, Amount: 0}, "amount must be > 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CreateInvoice(tc.inv)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("got %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestCreateInvoice_OK(t *testing.T) {
	setupInvoiceTestDB(t)
	inv := &Invoice{
		UserID:        42,
		ApplicantType: InvoiceApplicantPersonal,
		Title:         "Alice",
		Email:         "a@b.com",
		InvoiceType:   InvoiceTypeVATNormal,
		Amount:        100.5,
		AppliedAt:     common.GetTimestamp(),
	}
	if err := CreateInvoice(inv); err != nil {
		t.Fatalf("create: %v", err)
	}
	if inv.Id == 0 {
		t.Fatal("id not assigned")
	}
	if inv.Status != InvoiceStatusPending {
		t.Fatalf("default status = %q, want pending", inv.Status)
	}
}

func TestSumInvoicedAmount(t *testing.T) {
	setupInvoiceTestDB(t)
	for _, s := range []struct {
		amt    float64
		status string
	}{
		{10, InvoiceStatusPending},
		{20, InvoiceStatusIssuing},
		{30, InvoiceStatusIssued},
		{999, InvoiceStatusRejected}, // 不计入
	} {
		inv := &Invoice{
			UserID: 1, ApplicantType: InvoiceApplicantPersonal,
			Title: "x", Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal,
			Amount: s.amt, Status: s.status,
		}
		inv.AppliedAt = common.GetTimestamp()
		if err := DB.Create(inv).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, err := SumInvoicedAmount(1, InvoiceTopupSourceTopUps)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if got != 60 {
		t.Fatalf("sum = %v, want 60", got)
	}
}

func TestHasInFlightInvoice(t *testing.T) {
	setupInvoiceTestDB(t)
	got, err := HasInFlightInvoice(1)
	if err != nil || got {
		t.Fatalf("empty: got %v err %v", got, err)
	}
	DB.Create(&Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Title: "x",
		Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal, Amount: 10,
		Status: InvoiceStatusIssued, AppliedAt: common.GetTimestamp()})
	got, _ = HasInFlightInvoice(1)
	if got {
		t.Fatalf("issued shouldn't count")
	}
	DB.Create(&Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Title: "y",
		Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal, Amount: 10,
		Status: InvoiceStatusPending, AppliedAt: common.GetTimestamp()})
	got, _ = HasInFlightInvoice(1)
	if !got {
		t.Fatalf("pending should count")
	}
}

func TestTransitionInvoiceStatus_OptimisticLock(t *testing.T) {
	setupInvoiceTestDB(t)
	inv := &Invoice{UserID: 1, ApplicantType: InvoiceApplicantPersonal, Title: "x",
		Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal, Amount: 10,
		AppliedAt: common.GetTimestamp()}
	if err := CreateInvoice(inv); err != nil {
		t.Fatalf("create: %v", err)
	}
	ok, err := TransitionInvoiceStatus(inv.Id, InvoiceStatusPending, InvoiceStatusIssuing, nil)
	if err != nil || !ok {
		t.Fatalf("first transition: ok=%v err=%v", ok, err)
	}
	ok, err = TransitionInvoiceStatus(inv.Id, InvoiceStatusPending, InvoiceStatusIssuing, nil)
	if err != nil || ok {
		t.Fatalf("second transition should fail: ok=%v err=%v", ok, err)
	}
}

func TestSumTopUpSuccessMoney(t *testing.T) {
	setupInvoiceTestDB(t)
	// TopUp already migrated by TestMain. Just truncate user 1's rows.
	DB.Where("user_id = ?", 1).Delete(&TopUp{})

	DB.Create(&TopUp{UserId: 1, Money: 100, Status: common.TopUpStatusSuccess, TradeNo: "tinv1"})
	DB.Create(&TopUp{UserId: 1, Money: 200, Status: common.TopUpStatusSuccess, TradeNo: "tinv2"})
	DB.Create(&TopUp{UserId: 1, Money: 500, Status: common.TopUpStatusPending, TradeNo: "tinv3"})

	got, err := SumTopUpSuccessMoney(1)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if got != 300 {
		t.Fatalf("got %v want 300", got)
	}
}

func TestListInvoicesForUser_Pagination(t *testing.T) {
	setupInvoiceTestDB(t)
	for i := 0; i < 5; i++ {
		inv := &Invoice{UserID: 7, ApplicantType: InvoiceApplicantPersonal,
			Title: "u", Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal,
			Amount: float64(i + 1), AppliedAt: int64(i + 1)}
		if err := CreateInvoice(inv); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	items, total, err := ListInvoicesForUser(7, 1, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 5 || len(items) != 2 {
		t.Fatalf("got total=%d items=%d, want 5/2", total, len(items))
	}
	if items[0].Amount != 5 {
		t.Fatalf("first amount = %v, want 5", items[0].Amount)
	}
}

func TestAdminListInvoices_Filter(t *testing.T) {
	setupInvoiceTestDB(t)
	mk := func(uid int, status string, amt float64) {
		inv := &Invoice{UserID: uid, ApplicantType: InvoiceApplicantPersonal,
			Title: "x", Email: "a@b.com", InvoiceType: InvoiceTypeVATNormal,
			Amount: amt, Status: status, AppliedAt: common.GetTimestamp()}
		DB.Create(inv)
	}
	mk(1, InvoiceStatusPending, 10)
	mk(2, InvoiceStatusIssued, 20)
	mk(1, InvoiceStatusRejected, 30)

	items, total, err := AdminListInvoices(InvoiceListFilter{
		Statuses: []string{InvoiceStatusPending, InvoiceStatusIssued},
	}, 1, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("got total=%d, want 2", total)
	}
}
