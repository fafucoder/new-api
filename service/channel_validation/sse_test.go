package channel_validation

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A canonical Anthropic SSE stream containing one thinking block (with a
// signature delivered via signature_delta), one text block, and a
// terminating message_delta with stop_reason=end_turn.
const sampleAnthropicSSE = `event: message_start
data: {"type":"message_start","message":{"id":"msg_abc123","type":"message","role":"assistant","model":"claude-sonnet-4-5-20250929","content":[],"stop_reason":null,"usage":{"input_tokens":12,"cache_creation_input_tokens":1280,"cache_read_input_tokens":0,"output_tokens":1}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Let me consider this carefully."}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"EuYBC"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"kAJlz="}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Hello, world."}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":42}}

event: message_stop
data: {"type":"message_stop"}

`

func TestReadSSE_RecordsThinkingAndSignature(t *testing.T) {
	out := &StepOutcome{}
	acc := newSseAccumulator(out)
	if err := readSSE(strings.NewReader(sampleAnthropicSSE), acc); err != nil {
		t.Fatalf("readSSE failed: %v", err)
	}
	acc.finalise()

	if !out.ThinkingBlockSeen {
		t.Error("expected ThinkingBlockSeen=true")
	}
	if out.ThinkingFull != "Let me consider this carefully." {
		t.Errorf("ThinkingFull mismatch, got %q", out.ThinkingFull)
	}
	// signature should concatenate the two signature_delta payloads
	if out.SignatureFull != "EuYBCkAJlz=" {
		t.Errorf("SignatureFull mismatch, got %q", out.SignatureFull)
	}
	if !out.SignatureFromDelta {
		t.Error("expected SignatureFromDelta=true (came via signature_delta)")
	}
	if !out.MessageDeltaSeen {
		t.Error("expected MessageDeltaSeen=true")
	}
	if out.MessageDeltaStopReason != "end_turn" {
		t.Errorf("stop_reason mismatch, got %q", out.MessageDeltaStopReason)
	}
	if out.StopReasonIsMaxTokens {
		t.Error("end_turn must not flag StopReasonIsMaxTokens")
	}
	if out.RespondedModel != "claude-sonnet-4-5-20250929" {
		t.Errorf("RespondedModel mismatch, got %q", out.RespondedModel)
	}
	if out.ResponseID != "msg_abc123" {
		t.Errorf("ResponseID mismatch, got %q", out.ResponseID)
	}
	if !out.HasCacheCreationDetail {
		t.Error("expected HasCacheCreationDetail=true (usage carried cache_creation_input_tokens)")
	}
	if !out.HasCacheReadDetail {
		t.Error("expected HasCacheReadDetail=true (usage carried cache_read_input_tokens)")
	}
	if out.OutputTextPreview != "Hello, world." {
		t.Errorf("OutputTextPreview mismatch, got %q", out.OutputTextPreview)
	}
}

func TestReadSSE_HandlesErrorEvent(t *testing.T) {
	const errStream = `event: error
data: {"type":"error","error":{"type":"invalid_request_error","message":"signature is invalid"}}

`
	out := &StepOutcome{}
	acc := newSseAccumulator(out)
	if err := readSSE(strings.NewReader(errStream), acc); err != nil {
		t.Fatal(err)
	}
	acc.finalise()
	if !out.SseErrorEventSeen {
		t.Error("expected SseErrorEventSeen=true")
	}
	if out.SseErrorEventMessage != "signature is invalid" {
		t.Errorf("expected error message captured, got %q", out.SseErrorEventMessage)
	}
}

func TestReadSSE_HandlesMaxTokensStopReason(t *testing.T) {
	const stream = `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"max_tokens"},"usage":{"output_tokens":100}}

`
	out := &StepOutcome{}
	acc := newSseAccumulator(out)
	_ = readSSE(strings.NewReader(stream), acc)
	acc.finalise()
	if !out.StopReasonIsMaxTokens {
		t.Error("expected StopReasonIsMaxTokens=true")
	}
}

// TestPerformRequest_HappyPath uses an httptest server playing back a
// canonical Anthropic SSE stream. Verifies end-to-end that performRequest
// populates the outcome correctly and computes OK=true.
func TestPerformRequest_HappyPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header to be forwarded, got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Errorf("expected Anthropic-Version header, got %q", r.Header.Get("Anthropic-Version"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, strings.NewReader(sampleAnthropicSSE))
	}))
	defer server.Close()

	client := newHTTPClient()
	body := bytes.NewBufferString(`{"model":"claude-sonnet-4-5","stream":true}`).Bytes()
	out := performRequest(context.Background(), client, server.URL, "test-key", body, StepKindBaseline)
	if !out.OK {
		t.Fatalf("expected OK=true, got: %+v", out)
	}
	if out.Status != http.StatusOK {
		t.Errorf("expected status 200, got %d", out.Status)
	}
	if !out.IsSSE {
		t.Error("expected IsSSE=true")
	}
	if out.SignatureChars == 0 {
		t.Error("expected signature to be captured")
	}
}

// TestPerformRequest_HTTPErrorYieldsNotOK exercises the path where the
// upstream returns a non-2xx; the outcome should record the status and
// flag OK=false. Body content should still be captured into RawExcerpt
// for the tamper-detection logic.
func TestPerformRequest_HTTPErrorYieldsNotOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"invalid_signature"}}`)
	}))
	defer server.Close()

	client := newHTTPClient()
	out := performRequest(context.Background(), client, server.URL, "k", []byte("{}"), StepKindTamper)
	if out.OK {
		t.Error("expected OK=false on 400")
	}
	if out.Status != 400 {
		t.Errorf("expected status 400, got %d", out.Status)
	}
	if !strings.Contains(strings.ToLower(out.RawExcerpt), "invalid_signature") {
		t.Errorf("expected 'invalid_signature' to appear in raw excerpt, got %q", out.RawExcerpt)
	}
}

func TestTamperWasRejected_400_True(t *testing.T) {
	o := &StepOutcome{Status: 400}
	if !tamperWasRejected(o) {
		t.Error("HTTP 400 should count as rejection")
	}
}

func TestTamperWasRejected_RawExcerptInvalidSignature_True(t *testing.T) {
	o := &StepOutcome{Status: 500, RawExcerpt: `{"error":"invalid_signature in payload"}`}
	if !tamperWasRejected(o) {
		t.Error("raw_excerpt mentioning invalid_signature must count as rejection")
	}
}

func TestTamperWasRejected_200_False(t *testing.T) {
	o := &StepOutcome{Status: 200, MessageDeltaSeen: true, OK: true}
	if tamperWasRejected(o) {
		t.Error("HTTP 200 must not count as rejection (proxy ignored the tamper)")
	}
}

func TestBuildChecks_RequiredChecksPresent(t *testing.T) {
	steps := []StepOutcome{
		{
			Kind: StepKindBaseline, OK: true, Status: 200, IsSSE: true,
			MessageDeltaSeen: true, ThinkingBlockSeen: true,
			ThinkingChars: 42, SignatureChars: 16, RespondedModel: "claude-sonnet-4-5",
			ThinkingFull: "x", SignatureFull: "ABCDEFGHIJKLMNOP",
		},
		{Kind: StepKindMultiTurn, OK: true, Status: 200, IsSSE: true, MessageDeltaSeen: true},
		{Kind: StepKindTamper, Status: 400}, // rejected
	}
	checks := buildChecks(steps)

	wantKeys := []string{
		"baseline.request_ok",
		"baseline.sse_stream",
		"baseline.model_echo",
		"thinking.block_present",
		"thinking.signature_present",
		"multi_turn.accepts_own_signature",
		"tamper.rejected",
	}
	got := make(map[string]bool, len(checks))
	for _, c := range checks {
		got[c.Key] = c.Pass
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("expected check %q to be emitted", k)
		}
	}
	for _, k := range wantKeys {
		if !got[k] {
			t.Errorf("expected check %q to pass on the happy-path fixture", k)
		}
	}
}

func TestComputeVerdict_RealOnHappyPath(t *testing.T) {
	steps := []StepOutcome{{Kind: StepKindBaseline, OK: true}}
	checks := []Check{
		{Key: "baseline.request_ok", Required: true, Pass: true},
		{Key: "tamper.rejected", Required: true, Pass: true},
	}
	verdict, ok, _ := computeVerdict(steps, checks)
	if verdict != "real" || !ok {
		t.Errorf("expected (real, true), got (%q, %v)", verdict, ok)
	}
}

func TestComputeVerdict_SuspiciousWhenTamperAccepted(t *testing.T) {
	steps := []StepOutcome{{Kind: StepKindBaseline, OK: true}}
	checks := []Check{
		{Key: "baseline.request_ok", Required: true, Pass: true},
		{Key: "thinking.signature_present", Required: true, Pass: true},
		{Key: "tamper.rejected", Required: true, Pass: false},
	}
	verdict, ok, _ := computeVerdict(steps, checks)
	// tamper.rejected is required and failed → fake. (Suspicious is reserved
	// for the case where tamper was the ONLY required failure; if you
	// classify by required-fail count alone, this is fake. Either way the
	// suite must not return "real".)
	if verdict == "real" || ok {
		t.Errorf("must not return real verdict when tamper was accepted, got %q", verdict)
	}
}

func TestComputeVerdict_UnknownWhenBaselineFails(t *testing.T) {
	steps := []StepOutcome{{Kind: StepKindBaseline, OK: false}}
	verdict, ok, _ := computeVerdict(steps, nil)
	if verdict != "unknown" || ok {
		t.Errorf("expected unknown when baseline fails, got %q", verdict)
	}
}
