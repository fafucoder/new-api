package channel_validation

import (
	"fmt"
	"strings"
)

const (
	cachePadBegin = "[AIO_CACHE_PAD_BEGIN]"
	cachePadEnd   = "[AIO_CACHE_PAD_END]"
	cachePadWord  = "cachepad"
)

// inferCacheMinTokensForModel returns the minimum tokens required to trigger
// Anthropic's prompt cache for a given model family. Smaller models have
// lower thresholds. Defaults to the safe 1024 floor.
//
// Numbers come from the reference implementation and Anthropic's docs.
func inferCacheMinTokensForModel(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "haiku-4-5"), strings.Contains(m, "haiku-4.5"):
		return 4096
	case strings.Contains(m, "haiku-3-5"), strings.Contains(m, "haiku-3.5"),
		strings.Contains(m, "haiku-3"), strings.Contains(m, "haiku"):
		return 2048
	default:
		return 1024
	}
}

// buildCachePadText assembles the padding blob written into the cached
// system message. min_tokens+256 cachepad words yields ~5KB of text that
// reliably crosses the cache-creation threshold while staying easy to
// identify in raw responses.
func buildCachePadText(minTokens int) string {
	count := minTokens + 256
	var b strings.Builder
	b.Grow(len(cachePadBegin) + len(cachePadEnd) + count*(len(cachePadWord)+1) + 4)
	b.WriteString(cachePadBegin)
	b.WriteByte('\n')
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(cachePadWord)
	}
	b.WriteByte('\n')
	b.WriteString(cachePadEnd)
	return b.String()
}

// applyCachePaddingToBody appends a cache-control'd padding block to the
// request body's system message. Mutates body in place. Returns true when
// padding was applied (i.e. the body had a recognisable system message).
//
// Anthropic's API accepts `system` as either a string or an array of
// content blocks; this helper handles both forms by normalising to the
// array form with an ephemeral cache_control marker on the padding block.
func applyCachePaddingToBody(body map[string]any, minTokens int) bool {
	if body == nil {
		return false
	}
	padText := buildCachePadText(minTokens)
	padBlock := map[string]any{
		"type":          "text",
		"text":          padText,
		"cache_control": map[string]any{"type": "ephemeral"},
	}

	sysRaw, hasSys := body["system"]
	if !hasSys {
		body["system"] = []any{padBlock}
		return true
	}

	switch sys := sysRaw.(type) {
	case string:
		// Preserve original system text, then append cache-pad block.
		body["system"] = []any{
			map[string]any{"type": "text", "text": sys},
			padBlock,
		}
		return true
	case []any:
		body["system"] = append(sys, padBlock)
		return true
	default:
		// Unknown shape — wrap defensively.
		body["system"] = []any{
			map[string]any{"type": "text", "text": fmt.Sprintf("%v", sys)},
			padBlock,
		}
		return true
	}
}
