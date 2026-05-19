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
