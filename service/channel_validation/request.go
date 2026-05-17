package channel_validation

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// buildBaselineRequest constructs the Step1 request body. The probe is a
// minimal, fast prompt that asks for thinking — that's the smallest test
// surface that distinguishes a real Claude (returns thinking + signature)
// from a generic chat-completion proxy.
func buildBaselineRequest(model string, maxTokens int, forcePadding bool) map[string]any {
	if maxTokens <= 0 {
		maxTokens = 512
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"stream":     true,
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": 1024,
		},
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": "Think briefly, then reply with one short sentence. " +
					"This is an automated provider authenticity probe.",
			},
		},
	}
	if forcePadding {
		applyCachePaddingToBody(body, inferCacheMinTokensForModel(model))
	}
	return body
}

// buildMultiTurnRequest is Step2: feed the model its own prior thinking +
// signature as an assistant message, then ask it to continue. Real
// Claude accepts its own signed thinking; many fakes either reject it,
// emit a generic error, or hallucinate without acknowledging the signed
// block.
func buildMultiTurnRequest(model string, maxTokens int, step2Prompt, priorThinking, priorSignature, priorTextFallback string) (map[string]any, error) {
	if strings.TrimSpace(priorThinking) == "" || strings.TrimSpace(priorSignature) == "" {
		return nil, errors.New("multi-turn step needs both thinking and signature from baseline")
	}
	if maxTokens <= 0 {
		maxTokens = 512
	}
	if strings.TrimSpace(step2Prompt) == "" {
		step2Prompt = DefaultStep2Prompt
	}

	assistantContent := []any{
		map[string]any{
			"type":      "thinking",
			"thinking":  priorThinking,
			"signature": priorSignature,
		},
	}
	if strings.TrimSpace(priorTextFallback) != "" {
		assistantContent = append(assistantContent, map[string]any{
			"type": "text",
			"text": priorTextFallback,
		})
	}

	return map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"stream":     true,
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": 1024,
		},
		"messages": []any{
			map[string]any{"role": "user", "content": "Continue."},
			map[string]any{"role": "assistant", "content": assistantContent},
			map[string]any{"role": "user", "content": step2Prompt},
		},
	}, nil
}

// tamperSignature flips two characters in the middle of the signature so
// the signed thinking block is no longer authentic. Real Claude detects
// the tamper and returns 400 invalid_signature; faulty proxies will
// either accept it (suspicious) or fail with a generic 5xx (suspicious
// in a different way).
//
// The tamper is deterministic and reversible (idempotent within a run),
// so behaviour is reproducible. Returns the empty string when the input
// is too short to safely tamper.
func tamperSignature(sig string) string {
	if len(sig) < 8 {
		return ""
	}
	mid := len(sig) / 2
	runes := []rune(sig)
	if mid >= len(runes) {
		mid = len(runes) - 1
	}
	flip := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			if r == 'z' {
				return 'a'
			}
			return r + 1
		case r >= 'A' && r <= 'Z':
			if r == 'Z' {
				return 'A'
			}
			return r + 1
		case r >= '0' && r <= '9':
			if r == '9' {
				return '0'
			}
			return r + 1
		default:
			return r
		}
	}
	runes[mid] = flip(runes[mid])
	if mid+1 < len(runes) {
		runes[mid+1] = flip(runes[mid+1])
	}
	return string(runes)
}

// marshalBody serialises a Go map to a JSON byte slice through the
// project's common wrapper (per CLAUDE.md Rule 1).
func marshalBody(body map[string]any) ([]byte, error) {
	return common.Marshal(body)
}
