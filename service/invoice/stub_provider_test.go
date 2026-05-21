package invoice

import (
	"context"
	"strings"
	"testing"
)

func TestStubProvider_Issue(t *testing.T) {
	p := &stubProvider{}
	if p.Name() != "stub" {
		t.Fatalf("name = %q", p.Name())
	}
	got, err := p.Issue(context.Background(), &IssueRequest{
		InvoiceID: 42, UserID: 1, ApplicantType: "personal",
		Title: "Alice", Email: "a@b.com",
		InvoiceType: "vat_normal", Amount: 100,
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !strings.HasPrefix(got.ProviderInvoiceNo, "STUB-42-") {
		t.Fatalf("invoice no = %q", got.ProviderInvoiceNo)
	}
	if !strings.Contains(got.RawResponse, "\"provider\":\"stub\"") {
		t.Fatalf("raw = %q", got.RawResponse)
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	p, err := Get("stub")
	if err != nil {
		t.Fatalf("get stub: %v", err)
	}
	if p.Name() != "stub" {
		t.Fatalf("name = %q", p.Name())
	}
	if _, err := Get("does-not-exist"); err == nil {
		t.Fatalf("expected error for unknown provider")
	}
}
