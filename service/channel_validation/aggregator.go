package channel_validation

import (
	"fmt"
	"strings"
)

// buildChecks aggregates per-step outcomes into the flat list of pass/fail
// rows the UI renders. Each check has a stable Key for analytics; Label
// is English by default and translated client-side.
func buildChecks(steps []StepOutcome) []Check {
	baseline := findStep(steps, StepKindBaseline)
	multi := findStep(steps, StepKindMultiTurn)
	tamper := findStep(steps, StepKindTamper)
	cross := findStep(steps, StepKindCrossCheck)

	checks := make([]Check, 0, 16)

	// --- Baseline-level checks (always run, always required) ---
	checks = append(checks, Check{
		Key:      "baseline.request_ok",
		Label:    "Baseline request succeeded",
		Required: true,
		Pass:     baseline != nil && baseline.OK,
		Detail:   detailForBaselineRequest(baseline),
	})
	checks = append(checks, Check{
		Key:      "baseline.sse_stream",
		Label:    "Returned a complete SSE stream",
		Required: true,
		Pass:     baseline != nil && baseline.IsSSE && baseline.MessageDeltaSeen,
		Detail:   detailForSSE(baseline),
	})
	checks = append(checks, Check{
		Key:      "baseline.model_echo",
		Label:    "Echoed back a Claude model identifier",
		Required: true,
		Pass:     baseline != nil && looksLikeClaudeModel(baseline.RespondedModel),
		Detail:   detailForModelEcho(baseline),
	})

	// --- The Claude-specific signals ---
	checks = append(checks, Check{
		Key:      "thinking.block_present",
		Label:    "Emitted a thinking block",
		Required: true,
		Pass:     baseline != nil && baseline.ThinkingBlockSeen && baseline.ThinkingChars > 0,
		Detail:   detailForThinking(baseline),
	})
	checks = append(checks, Check{
		Key:      "thinking.signature_present",
		Label:    "Thinking block carried a signature",
		Required: true,
		Pass:     baseline != nil && baseline.SignatureChars > 0,
		Detail:   detailForSignature(baseline),
	})

	// --- Multi-turn signature replay ---
	if multi != nil {
		checks = append(checks, Check{
			Key:      "multi_turn.accepts_own_signature",
			Label:    "Accepted its own signed thinking on replay",
			Required: true,
			Pass:     multi.OK && multi.MessageDeltaSeen,
			Detail:   detailForMultiTurn(multi),
		})
	} else {
		checks = append(checks, Check{
			Key:        "multi_turn.accepts_own_signature",
			Label:      "Accepted its own signed thinking on replay",
			Required:   true,
			Pass:       false,
			NotChecked: true,
			Detail:     "Step2 skipped (baseline did not produce a usable signature)",
		})
	}

	// --- Signature tamper negative test ---
	if tamper != nil {
		// A pass here means the upstream REJECTED the tampered signature
		// (returned 4xx or surfaced "invalid_signature"). Real Claude does;
		// a passthrough proxy that doesn't actually verify will accept it.
		tampered := tamper
		rejected := tamperWasRejected(tampered)
		checks = append(checks, Check{
			Key:      "tamper.rejected",
			Label:    "Rejected a tampered signature",
			Required: true,
			Pass:     rejected,
			Detail:   detailForTamper(tampered, rejected),
		})
	} else {
		checks = append(checks, Check{
			Key:        "tamper.rejected",
			Label:      "Rejected a tampered signature",
			Required:   false,
			Pass:       false,
			NotChecked: true,
			Detail:     "Tamper negative test was not run for this validation",
		})
	}

	// --- Cache breakdown (Anthropic only exposes this when caching is enabled) ---
	if baseline != nil && (baseline.HasCacheCreationDetail || baseline.HasCacheReadDetail) {
		checks = append(checks, Check{
			Key:      "usage.cache_detail",
			Label:    "Usage payload includes Anthropic cache fields",
			Required: false,
			Pass:     true,
			Detail:   "cache_creation_input_tokens or cache_read_input_tokens present in usage",
		})
	} else {
		checks = append(checks, Check{
			Key:        "usage.cache_detail",
			Label:      "Usage payload includes Anthropic cache fields",
			Required:   false,
			Pass:       false,
			NotChecked: !baseline.cachePadActive(),
			Detail:     "Enable cache padding to verify; absent fields are not by themselves diagnostic",
		})
	}

	// --- Cross-channel verification ---
	if cross != nil {
		checks = append(checks, Check{
			Key:      "cross.signature_accepted",
			Label:    "Reference channel accepted the same signature",
			Required: false,
			Pass:     cross.OK && cross.MessageDeltaSeen,
			Detail:   detailForCross(cross),
		})
	}

	return checks
}

// computeVerdict reduces the checks list to a single verdict label + summary.
// Real → all required checks pass. Suspicious → required all pass but
// optional/tamper fails. Fake → any required check fails. Unknown → step1
// itself failed (no signal to interpret).
func computeVerdict(steps []StepOutcome, checks []Check) (verdict string, ok bool, summary string) {
	baseline := findStep(steps, StepKindBaseline)
	if baseline == nil || !baseline.OK {
		return "unknown", false, "Baseline probe failed; cannot determine authenticity"
	}

	requiredFail := 0
	requiredPass := 0
	tamperFail := false
	for _, c := range checks {
		if c.NotChecked {
			continue
		}
		if c.Required {
			if c.Pass {
				requiredPass++
			} else {
				requiredFail++
			}
		}
		if c.Key == "tamper.rejected" && !c.Pass && !c.NotChecked {
			tamperFail = true
		}
	}

	switch {
	case requiredFail == 0 && !tamperFail:
		return "real", true, fmt.Sprintf("All %d required checks passed", requiredPass)
	case requiredFail == 0 && tamperFail:
		return "suspicious", false, "All baseline checks pass but the upstream accepted a tampered signature"
	default:
		return "fake", false, fmt.Sprintf("%d required check(s) failed", requiredFail)
	}
}

// --- Helpers ---

func findStep(steps []StepOutcome, kind StepKind) *StepOutcome {
	for i := range steps {
		if steps[i].Kind == kind {
			return &steps[i]
		}
	}
	return nil
}

// looksLikeClaudeModel checks whether the responded model name contains the
// Anthropic family marker. Real Claude always echoes a name like
// "claude-sonnet-4-5-..."; a generic proxy often echoes the requested name
// verbatim (which is fine), but a chat-completion fake may return the
// underlying gpt model.
func looksLikeClaudeModel(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	return strings.Contains(lower, "claude")
}

// tamperWasRejected returns true if the upstream rejected the tampered
// signature in a way that proves it actually verifies the signature. The
// strongest signal is HTTP 400 + an error mentioning "signature"; a
// generic 5xx is a weaker signal.
func tamperWasRejected(o *StepOutcome) bool {
	if o == nil {
		return false
	}
	// Explicit rejection on the wire
	if o.Status == 400 || o.Status == 401 || o.Status == 403 {
		return true
	}
	// SSE error events naming the signature
	if o.SseErrorEventSeen {
		msg := strings.ToLower(o.SseErrorEventMessage)
		if strings.Contains(msg, "signature") || strings.Contains(msg, "thinking") {
			return true
		}
	}
	// Raw-excerpt fallback for body-encoded errors
	raw := strings.ToLower(o.RawExcerpt)
	if strings.Contains(raw, "invalid_signature") || strings.Contains(raw, "invalid_thinking_signature") {
		return true
	}
	return false
}

// detailForX helpers produce short human-readable strings for each check.
// They lean on the corresponding StepOutcome but degrade gracefully when
// the step is nil.

func detailForBaselineRequest(o *StepOutcome) string {
	if o == nil {
		return "Baseline step did not run"
	}
	if o.OK {
		return fmt.Sprintf("HTTP %d, %dms", o.Status, o.DurationMs)
	}
	if o.ErrorText != "" {
		return o.ErrorText
	}
	return fmt.Sprintf("HTTP %d", o.Status)
}

func detailForSSE(o *StepOutcome) string {
	if o == nil {
		return "Baseline step did not run"
	}
	if !o.IsSSE {
		return "Response Content-Type was " + o.ContentType + " (expected event-stream)"
	}
	if !o.MessageDeltaSeen {
		return "Stream ended without a message_delta event"
	}
	if o.StreamReadError != "" {
		return "Stream read error: " + o.StreamReadError
	}
	return "Complete SSE stream; stop_reason=" + o.MessageDeltaStopReason
}

func detailForModelEcho(o *StepOutcome) string {
	if o == nil {
		return ""
	}
	if o.RespondedModel == "" {
		return "Response did not contain a model identifier"
	}
	return "responded_model=" + o.RespondedModel
}

func detailForThinking(o *StepOutcome) string {
	if o == nil {
		return ""
	}
	if !o.ThinkingBlockSeen {
		return "No content_block of type 'thinking' was emitted"
	}
	return fmt.Sprintf("%d characters of thinking", o.ThinkingChars)
}

func detailForSignature(o *StepOutcome) string {
	if o == nil {
		return ""
	}
	if o.SignatureChars == 0 {
		return "Thinking block had no signature attached"
	}
	via := "via content_block_start"
	if o.SignatureFromDelta {
		via = "via signature_delta stream"
	}
	return fmt.Sprintf("%d-char signature %s", o.SignatureChars, via)
}

func detailForMultiTurn(o *StepOutcome) string {
	if o == nil {
		return ""
	}
	if !o.OK {
		if o.ErrorText != "" {
			return "Replay failed: " + o.ErrorText
		}
		return fmt.Sprintf("Replay failed (HTTP %d)", o.Status)
	}
	return fmt.Sprintf("Replay succeeded (%dms, stop_reason=%s)", o.DurationMs, o.MessageDeltaStopReason)
}

func detailForTamper(o *StepOutcome, rejected bool) string {
	if o == nil {
		return ""
	}
	if rejected {
		if o.SseErrorEventMessage != "" {
			return fmt.Sprintf("Rejected (HTTP %d): %s", o.Status, o.SseErrorEventMessage)
		}
		return fmt.Sprintf("Rejected (HTTP %d)", o.Status)
	}
	return fmt.Sprintf("Upstream ACCEPTED a tampered signature (HTTP %d) — this is what fake proxies do", o.Status)
}

func detailForCross(o *StepOutcome) string {
	if o == nil {
		return ""
	}
	if o.OK {
		return "Reference channel accepted the signature — primary signature is genuine"
	}
	return "Reference channel rejected the signature; signature may be malformed or domain-specific"
}

// cachePadActive is a small helper for the NotChecked logic above; on a nil
// receiver it returns false so the check correctly reports "not run".
func (o *StepOutcome) cachePadActive() bool {
	return o != nil && o.CachePadApplied
}
