package openai

import (
	"github.com/QuantumNous/new-api/common"
)

// ConvertKimiResponse 将非官方 Kimi 渠道（如 OpenRouter 等任意上游）返回的
// 非流式响应对齐为官方 Kimi 响应格式：仅保留官方字段（白名单），去除多余字段。
func ConvertKimiResponse(responseBody []byte) ([]byte, error) {
	var body map[string]any
	if err := common.Unmarshal(responseBody, &body); err != nil {
		return responseBody, err
	}

	convertKimiBody(body)

	return common.Marshal(body)
}

// ConvertKimiStreamChunk 将非官方 Kimi 渠道返回的流式分片对齐为官方 Kimi 格式。
func ConvertKimiStreamChunk(data string) string {
	var body map[string]any
	if err := common.Unmarshal(common.StringToByteSlice(data), &body); err != nil {
		return data
	}

	convertKimiBody(body)

	result, err := common.Marshal(body)
	if err != nil {
		return data
	}
	return string(result)
}

// convertKimiBody 采用白名单方式裁剪响应体：只保留官方 Kimi 响应中存在的字段，
// 其余字段（包括未知的新增字段）一律删除。
func convertKimiBody(body map[string]any) {
	// 顶层白名单
	keepOnly(body, "id", "object", "created", "model", "choices", "usage", "error")

	if choices, ok := body["choices"].([]any); ok {
		for _, choice := range choices {
			choiceMap, ok := choice.(map[string]any)
			if !ok {
				continue
			}
			// choice 白名单
			keepOnly(choiceMap, "index", "message", "delta", "finish_reason")

			if msg, ok := choiceMap["message"].(map[string]any); ok {
				convertKimiMessageFields(msg)
			}
			if delta, ok := choiceMap["delta"].(map[string]any); ok {
				convertKimiMessageFields(delta)
			}
		}
	}

	if usage, ok := body["usage"].(map[string]any); ok {
		convertKimiUsage(usage)
	}
}

// convertKimiMessageFields 对齐 message/delta 字段：
// 将上游的 reasoning 改名为官方的 reasoning_content，并只保留官方字段。
func convertKimiMessageFields(msg map[string]any) {
	// reasoning -> reasoning_content
	if reasoning, exists := msg["reasoning"]; exists {
		if _, hasReasoningContent := msg["reasoning_content"]; !hasReasoningContent {
			msg["reasoning_content"] = reasoning
		}
		delete(msg, "reasoning")
	}
	// message/delta 白名单
	keepOnly(msg, "role", "content", "reasoning_content", "tool_calls")
}

// convertKimiUsage 对齐 usage 字段。顶层保留官方字段并补齐顶层 cached_tokens；
// 同时保留 new-api 内部计费/日志所需的 prompt_tokens_details.cached_tokens 与
// completion_tokens_details.reasoning_tokens。
func convertKimiUsage(usage map[string]any) {
	var cachedTokens float64
	if ptd, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		if ct, ok := ptd["cached_tokens"].(float64); ok {
			cachedTokens = ct
		}
		usage["prompt_tokens_details"] = map[string]any{
			"cached_tokens": cachedTokens,
		}
	}
	// 若上游把 cached_tokens 直接放在顶层，用它兜底
	if cachedTokens == 0 {
		if ct, ok := usage["cached_tokens"].(float64); ok {
			cachedTokens = ct
		}
	}

	if ctd, ok := usage["completion_tokens_details"].(map[string]any); ok {
		var reasoningTokens float64
		if rt, ok := ctd["reasoning_tokens"].(float64); ok {
			reasoningTokens = rt
		}
		usage["completion_tokens_details"] = map[string]any{
			"reasoning_tokens": reasoningTokens,
		}
	}

	// usage 白名单
	keepOnly(usage, "prompt_tokens", "completion_tokens", "total_tokens",
		"cached_tokens", "prompt_tokens_details", "completion_tokens_details")

	// 顶层 cached_tokens 对齐官方
	usage["cached_tokens"] = cachedTokens
}

// keepOnly 删除 m 中不在 allowed 白名单内的所有键。
func keepOnly(m map[string]any, allowed ...string) {
	allow := make(map[string]struct{}, len(allowed))
	for _, k := range allowed {
		allow[k] = struct{}{}
	}
	for k := range m {
		if _, ok := allow[k]; !ok {
			delete(m, k)
		}
	}
}
