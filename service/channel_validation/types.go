// Package channel_validation runs deep "is this really Claude?" probes
// against a configured channel. The methodology mirrors the open-source
// reference at https://github.com/dyndynjyxa/aio-coding-hub: a 3-step
// suite exercises Claude-specific signals (thinking blocks, signed
// thinking, signature-tamper rejection, cache padding) that are hard for
// a generic proxy to forge.
package channel_validation

import "encoding/json"

// Constants tunable from a single place so workflow / response stay in sync.
const (
	// MaxBodyBytes caps how much of a single step's response we read. Real
	// Claude responses fit easily; the cap defends against malicious
	// upstreams that try to exhaust memory with an infinite stream.
	MaxBodyBytes = 512 * 1024

	// MaxRawExcerptBytes is how many bytes of the raw response we surface
	// to the UI for debugging.
	MaxRawExcerptBytes = 16 * 1024

	// MaxThinkingChars / MaxSignatureChars bound the per-step text we
	// retain for cross-step replay in Step2.
	MaxThinkingChars  = 120 * 1024
	MaxSignatureChars = 24 * 1024

	// MaxOutputPreviewChars is how many chars of assistant output text we
	// keep for the result panel's quick preview.
	MaxOutputPreviewChars = 4000

	// DefaultStep2Prompt is the user message in Step2 that asks the model
	// to acknowledge that the prior thinking+signature was preserved. A
	// real Claude responds in continuation; many fakes choke.
	DefaultStep2Prompt = "Respond with the exact string AIO_MULTI_TURN_OK on the first line, then briefly summarize the prior thinking."
)

// StepKind identifies which step of the suite produced an outcome.
type StepKind string

const (
	StepKindBaseline   StepKind = "baseline"   // Step1
	StepKindMultiTurn  StepKind = "multi_turn" // Step2 (preserved thinking+signature)
	StepKindTamper     StepKind = "tamper"     // Step3 (signature-tamper negative test)
	StepKindCrossCheck StepKind = "cross"      // Optional: replay Step2 against a different Claude channel
)

// StepOutcome captures everything observable from one HTTP/SSE round-trip
// to the upstream. Populated by execute.go + response.go.
type StepOutcome struct {
	Kind StepKind `json:"kind"`

	OK         bool   `json:"ok"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	ErrorText  string `json:"error,omitempty"`

	RequestedModel string `json:"requested_model,omitempty"`
	RespondedModel string `json:"responded_model,omitempty"`
	ResponseID     string `json:"response_id,omitempty"`
	ServiceTier    string `json:"service_tier,omitempty"`

	// Streaming signals
	ContentType                string `json:"content_type,omitempty"`
	IsSSE                      bool   `json:"is_sse"`
	StreamReadError            string `json:"stream_read_error,omitempty"`
	MessageDeltaSeen           bool   `json:"message_delta_seen"`
	MessageDeltaStopReason     string `json:"stop_reason,omitempty"`
	StopReasonIsMaxTokens      bool   `json:"stop_reason_is_max_tokens"`
	SseErrorEventSeen          bool   `json:"sse_error_event_seen"`
	SseErrorEventStatus        int    `json:"sse_error_event_status,omitempty"`
	SseErrorEventMessage       string `json:"sse_error_event_message,omitempty"`

	// Thinking + signature (the meat of Claude verification)
	ThinkingBlockSeen   bool   `json:"thinking_block_seen"`
	ThinkingChars       int    `json:"thinking_chars"`
	ThinkingFull        string `json:"thinking_full,omitempty"`
	SignatureChars      int    `json:"signature_chars"`
	SignatureFull       string `json:"signature_full,omitempty"`
	SignatureFromDelta  bool   `json:"signature_from_delta"`

	// Output text
	OutputTextChars   int    `json:"output_text_chars"`
	OutputTextPreview string `json:"output_text_preview,omitempty"`

	// Tool use / web search
	ServerToolUseSeen      bool     `json:"server_tool_use_seen"`
	WebSearchResultSeen    bool     `json:"web_search_tool_result_seen"`
	WebSearchResultURLs    []string `json:"web_search_result_urls,omitempty"`
	WebSearchRequestsCount int      `json:"web_search_requests_count,omitempty"`

	// Usage
	Usage json.RawMessage `json:"usage,omitempty"`

	// Cache breakdown (real Claude exposes cache_creation_input_tokens / cache_read_input_tokens)
	CachePadApplied       bool `json:"cache_pad_applied"`
	HasCacheCreationDetail bool `json:"has_cache_creation_detail"`
	HasCacheReadDetail    bool `json:"has_cache_read_detail"`

	// Raw response excerpt for debugging (first MaxRawExcerptBytes bytes)
	RawExcerpt string `json:"raw_excerpt,omitempty"`

	// Response headers (masked)
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
}

// Check is one aggregated verification dimension shown in the UI. Each
// derives from one or more StepOutcome fields.
type Check struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Required  bool   `json:"required"`
	Pass      bool   `json:"pass"`
	Detail    string `json:"detail,omitempty"`
	NotChecked bool  `json:"not_checked,omitempty"`
}

// ValidationResult is the top-level response returned to the frontend.
type ValidationResult struct {
	OK             bool   `json:"ok"`
	Verdict        string `json:"verdict"` // "real" | "suspicious" | "fake" | "unknown"
	Summary        string `json:"summary"`

	// RecordID is the persisted history record id once the controller
	// has saved this run. Zero on a fresh ValidationResult — populated
	// after Run returns, in the HTTP handler.
	RecordID int `json:"record_id,omitempty"`

	ChannelID      int    `json:"channel_id"`
	ChannelName    string `json:"channel_name"`
	ChannelType    int    `json:"channel_type"`
	BaseURL        string `json:"base_url"`
	RequestedModel string `json:"requested_model"`
	RespondedModel string `json:"responded_model,omitempty"`

	StartedAt   int64 `json:"started_at"`
	DurationMs  int64 `json:"duration_ms"`

	Steps  []StepOutcome `json:"steps"`
	Checks []Check       `json:"checks"`
}

// Options controls which steps run and how they behave.
type Options struct {
	Model      string `json:"model"`
	MaxTokens  int    `json:"max_tokens,omitempty"`

	// Step2 toggles
	RunMultiTurn   bool   `json:"run_multi_turn"`
	Step2Prompt    string `json:"step2_prompt,omitempty"`

	// Step3 toggles
	RunTamper      bool   `json:"run_tamper"`

	// Cross-provider verification: re-run Step2 against this other channel
	CrossChannelID int    `json:"cross_channel_id,omitempty"`

	// Force cache padding even when the model doesn't normally need it
	ForcePadding   bool   `json:"force_padding"`
}
