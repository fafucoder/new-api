package invoice

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

// stubProvider 永远成功的占位实现, 用于 v1 / 测试 / 本地开发。
// 返回的回执号形如 STUB-{invoice_id}-{unix_ts}, PDFURL 为空。
type stubProvider struct{}

func (s *stubProvider) Name() string { return "stub" }

func (s *stubProvider) Issue(ctx context.Context, req *IssueRequest) (*IssueResult, error) {
	ts := common.GetTimestamp()
	logger.LogInfo(ctx, fmt.Sprintf("invoice stub: issuing invoice=%d user=%d amount=%v",
		req.InvoiceID, req.UserID, req.Amount))
	no := fmt.Sprintf("STUB-%d-%d", req.InvoiceID, ts)
	raw := fmt.Sprintf(`{"provider":"stub","invoice_no":%q,"ts":%d}`, no, ts)
	return &IssueResult{
		ProviderInvoiceNo: no,
		PDFURL:            "",
		RawResponse:       raw,
	}, nil
}
