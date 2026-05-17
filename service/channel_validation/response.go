package channel_validation

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// sseAccumulator parses Anthropic-format SSE events and aggregates the
// signals we need to verify "is this really Claude?". One instance per
// step. Methods are NOT goroutine-safe — each step owns its own.
type sseAccumulator struct {
	out *StepOutcome

	// Thinking/signature buffers; trimmed to the type bounds when read.
	thinkingBuilder  strings.Builder
	signatureBuilder strings.Builder

	// Output text buffer for preview + count.
	outputBuilder strings.Builder

	// Block-type tracker keyed by index → block type ("thinking", "text",
	// "server_tool_use", "web_search_tool_result", ...).
	blockTypes map[int]string

	// Per-block in-progress signature buffer, in case the model emits the
	// signature via signature_delta on a thinking block.
	pendingSig map[int]*strings.Builder
}

func newSseAccumulator(out *StepOutcome) *sseAccumulator {
	return &sseAccumulator{
		out:        out,
		blockTypes: make(map[int]string),
		pendingSig: make(map[int]*strings.Builder),
	}
}

// dispatch parses one SSE event payload (the concatenated `data:` lines for
// one event) and updates accumulator state.
func (a *sseAccumulator) dispatch(payload []byte) {
	if len(payload) == 0 {
		return
	}
	var ev map[string]any
	if err := common.Unmarshal(payload, &ev); err != nil {
		// Not all SSE lines are JSON (heartbeats, comments). Silently skip.
		return
	}
	evType, _ := ev["type"].(string)
	switch evType {
	case "message_start":
		a.handleMessageStart(ev)
	case "content_block_start":
		a.handleBlockStart(ev)
	case "content_block_delta":
		a.handleBlockDelta(ev)
	case "content_block_stop":
		// content blocks complete; signatures emitted via delta finalise here.
		idx := intFrom(ev["index"])
		if pending, ok := a.pendingSig[idx]; ok {
			a.absorbSignature(pending.String(), true)
			delete(a.pendingSig, idx)
		}
	case "message_delta":
		a.handleMessageDelta(ev)
	case "error":
		a.handleErrorEvent(ev)
	}
}

func (a *sseAccumulator) handleMessageStart(ev map[string]any) {
	msg, _ := ev["message"].(map[string]any)
	if msg == nil {
		return
	}
	if id, ok := msg["id"].(string); ok && a.out.ResponseID == "" {
		a.out.ResponseID = id
	}
	if m, ok := msg["model"].(string); ok && a.out.RespondedModel == "" {
		a.out.RespondedModel = m
	}
	if tier, ok := msg["service_tier"].(string); ok {
		a.out.ServiceTier = tier
	}
	if usage, ok := msg["usage"].(map[string]any); ok {
		a.captureUsage(usage)
	}
}

func (a *sseAccumulator) handleBlockStart(ev map[string]any) {
	idx := intFrom(ev["index"])
	block, _ := ev["content_block"].(map[string]any)
	if block == nil {
		return
	}
	btype, _ := block["type"].(string)
	a.blockTypes[idx] = btype
	switch btype {
	case "thinking", "redacted_thinking":
		a.out.ThinkingBlockSeen = true
		if t, ok := block["thinking"].(string); ok && t != "" {
			a.appendThinking(t)
		}
		// Signature can be present already on the block_start payload, in
		// which case we absorb it as a "non-delta" signature.
		if sig, ok := block["signature"].(string); ok && sig != "" {
			a.absorbSignature(sig, false)
		}
	case "server_tool_use":
		a.out.ServerToolUseSeen = true
	case "web_search_tool_result":
		a.out.WebSearchResultSeen = true
		if content, ok := block["content"].([]any); ok {
			for _, item := range content {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if url, ok := m["url"].(string); ok && url != "" {
					a.out.WebSearchResultURLs = append(a.out.WebSearchResultURLs, url)
				}
			}
		}
	}
}

func (a *sseAccumulator) handleBlockDelta(ev map[string]any) {
	idx := intFrom(ev["index"])
	delta, _ := ev["delta"].(map[string]any)
	if delta == nil {
		return
	}
	dtype, _ := delta["type"].(string)
	switch dtype {
	case "thinking_delta":
		a.out.ThinkingBlockSeen = true
		if t, ok := delta["thinking"].(string); ok {
			a.appendThinking(t)
		}
	case "signature_delta":
		if s, ok := delta["signature"].(string); ok && s != "" {
			pending, ok := a.pendingSig[idx]
			if !ok {
				pending = &strings.Builder{}
				a.pendingSig[idx] = pending
			}
			pending.WriteString(s)
		}
	case "text_delta":
		if t, ok := delta["text"].(string); ok {
			a.appendOutputText(t)
		}
	case "input_json_delta":
		// Tool input — count as output bytes but don't surface as text preview.
	}
}

func (a *sseAccumulator) handleMessageDelta(ev map[string]any) {
	a.out.MessageDeltaSeen = true
	delta, _ := ev["delta"].(map[string]any)
	if delta != nil {
		if sr, ok := delta["stop_reason"].(string); ok && sr != "" {
			a.out.MessageDeltaStopReason = sr
			a.out.StopReasonIsMaxTokens = sr == "max_tokens"
		}
	}
	if usage, ok := ev["usage"].(map[string]any); ok {
		a.captureUsage(usage)
	}
}

func (a *sseAccumulator) handleErrorEvent(ev map[string]any) {
	a.out.SseErrorEventSeen = true
	if errObj, ok := ev["error"].(map[string]any); ok {
		if m, ok := errObj["message"].(string); ok {
			a.out.SseErrorEventMessage = m
		}
		if st, ok := errObj["status"].(float64); ok {
			a.out.SseErrorEventStatus = int(st)
		}
	}
}

func (a *sseAccumulator) appendThinking(s string) {
	if a.out.ThinkingChars >= MaxThinkingChars {
		return
	}
	remaining := MaxThinkingChars - a.out.ThinkingChars
	if len(s) > remaining {
		s = s[:remaining]
	}
	a.thinkingBuilder.WriteString(s)
	a.out.ThinkingChars += len(s)
}

func (a *sseAccumulator) absorbSignature(s string, fromDelta bool) {
	if len(s) == 0 {
		return
	}
	// Keep the longest signature observed (the reference repo's heuristic).
	if len(s) > a.out.SignatureChars {
		if len(s) > MaxSignatureChars {
			s = s[:MaxSignatureChars]
		}
		a.signatureBuilder.Reset()
		a.signatureBuilder.WriteString(s)
		a.out.SignatureChars = len(s)
		a.out.SignatureFromDelta = fromDelta
	}
}

func (a *sseAccumulator) appendOutputText(s string) {
	a.outputBuilder.WriteString(s)
	a.out.OutputTextChars += len(s)
}

func (a *sseAccumulator) captureUsage(usage map[string]any) {
	if _, ok := usage["cache_creation_input_tokens"]; ok {
		a.out.HasCacheCreationDetail = true
	}
	if _, ok := usage["cache_read_input_tokens"]; ok {
		a.out.HasCacheReadDetail = true
	}
	if raw, err := common.Marshal(usage); err == nil {
		a.out.Usage = raw
	}
}

// finalise transfers buffered text into the outcome and trims previews.
// Call exactly once after the SSE stream has ended.
func (a *sseAccumulator) finalise() {
	if a.thinkingBuilder.Len() > 0 {
		a.out.ThinkingFull = a.thinkingBuilder.String()
	}
	if a.signatureBuilder.Len() > 0 {
		a.out.SignatureFull = a.signatureBuilder.String()
	}
	if a.outputBuilder.Len() > 0 {
		full := a.outputBuilder.String()
		if len(full) > MaxOutputPreviewChars {
			a.out.OutputTextPreview = full[:MaxOutputPreviewChars]
		} else {
			a.out.OutputTextPreview = full
		}
	}
}

// intFrom is a defensive cast for JSON-decoded numeric fields. JSON
// integers arrive as float64 through encoding/json.
func intFrom(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
