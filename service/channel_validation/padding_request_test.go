package channel_validation

import (
	"strings"
	"testing"
)

func TestInferCacheMinTokensForModel(t *testing.T) {
	cases := map[string]int{
		"claude-haiku-4-5":     4096,
		"claude-haiku-4.5":     4096,
		"claude-haiku-3-5":     2048,
		"claude-haiku-3":       2048,
		"claude-haiku":         2048,
		"claude-sonnet-4-5":    1024,
		"claude-opus-4-5":      1024,
		"":                     1024,
	}
	for model, want := range cases {
		if got := inferCacheMinTokensForModel(model); got != want {
			t.Errorf("inferCacheMinTokensForModel(%q) = %d, want %d", model, got, want)
		}
	}
}

func TestBuildCachePadText_HasMarkersAndRepeats(t *testing.T) {
	const minTokens = 1024
	got := buildCachePadText(minTokens)
	if !strings.HasPrefix(got, cachePadBegin) {
		t.Errorf("padding text must start with %q, got prefix %q", cachePadBegin, got[:len(cachePadBegin)])
	}
	if !strings.HasSuffix(got, cachePadEnd) {
		t.Errorf("padding text must end with %q", cachePadEnd)
	}
	wantCount := minTokens + 256
	if got := strings.Count(got, cachePadWord); got != wantCount {
		t.Errorf("expected %d %q tokens, got %d", wantCount, cachePadWord, got)
	}
}

func TestApplyCachePaddingToBody_NoSystem(t *testing.T) {
	body := map[string]any{"model": "claude-sonnet-4-5", "messages": []any{}}
	if !applyCachePaddingToBody(body, 256) {
		t.Fatal("expected padding to be applied")
	}
	sys, ok := body["system"].([]any)
	if !ok || len(sys) != 1 {
		t.Fatalf("system must be a one-block array, got %T %v", body["system"], body["system"])
	}
	block, _ := sys[0].(map[string]any)
	if block == nil || block["type"] != "text" {
		t.Errorf("pad block must be type=text, got %v", block)
	}
	if _, ok := block["cache_control"]; !ok {
		t.Error("pad block must carry cache_control marker")
	}
}

func TestApplyCachePaddingToBody_StringSystem(t *testing.T) {
	body := map[string]any{"system": "You are helpful."}
	if !applyCachePaddingToBody(body, 128) {
		t.Fatal("expected padding to be applied to string-form system")
	}
	sys, ok := body["system"].([]any)
	if !ok || len(sys) != 2 {
		t.Fatalf("expected 2 blocks (original + pad), got %v", body["system"])
	}
	first, _ := sys[0].(map[string]any)
	if first["text"] != "You are helpful." {
		t.Errorf("original system text must be preserved as first block, got %v", first)
	}
}

func TestApplyCachePaddingToBody_ArraySystem(t *testing.T) {
	body := map[string]any{"system": []any{
		map[string]any{"type": "text", "text": "Existing instruction."},
	}}
	if !applyCachePaddingToBody(body, 64) {
		t.Fatal("expected padding to be applied to array-form system")
	}
	sys := body["system"].([]any)
	if len(sys) != 2 {
		t.Fatalf("expected 2 blocks after padding, got %d", len(sys))
	}
}

func TestTamperSignature_FlipsMidChars(t *testing.T) {
	sig := "AAAAAAAAAAAA" // 12 As, middle is index 6
	got := tamperSignature(sig)
	if got == sig {
		t.Fatal("tamper must return a different string")
	}
	if len(got) != len(sig) {
		t.Errorf("tamper must preserve length, got %d vs %d", len(got), len(sig))
	}
	// First half + tail unchanged, only middle two characters differ
	if got[:6] != sig[:6] {
		t.Errorf("prefix must be unchanged, got %q", got[:6])
	}
	if got[8:] != sig[8:] {
		t.Errorf("suffix must be unchanged, got %q", got[8:])
	}
}

func TestTamperSignature_TooShortReturnsEmpty(t *testing.T) {
	if got := tamperSignature("abc"); got != "" {
		t.Errorf("short input must return empty, got %q", got)
	}
}

func TestBuildBaselineRequest_StreamThinking(t *testing.T) {
	body := buildBaselineRequest("claude-sonnet-4-5", 256, false)
	if body["stream"] != true {
		t.Error("baseline body must set stream=true")
	}
	if body["model"] != "claude-sonnet-4-5" {
		t.Errorf("baseline body must echo requested model, got %v", body["model"])
	}
	thinking, ok := body["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Error("baseline body must enable thinking")
	}
}

func TestBuildMultiTurnRequest_IncludesPreservedThinking(t *testing.T) {
	body, err := buildMultiTurnRequest("claude-sonnet-4-5", 256, "", "the thought", "the-sig", "text fallback")
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := body["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	asst := messages[1].(map[string]any)
	content := asst["content"].([]any)
	thinkBlock := content[0].(map[string]any)
	if thinkBlock["thinking"] != "the thought" || thinkBlock["signature"] != "the-sig" {
		t.Errorf("assistant message must preserve thinking/signature, got %v", thinkBlock)
	}
}

func TestBuildMultiTurnRequest_NoSignatureReturnsError(t *testing.T) {
	if _, err := buildMultiTurnRequest("m", 0, "", "thought", "", ""); err == nil {
		t.Error("missing signature must surface an error")
	}
}
