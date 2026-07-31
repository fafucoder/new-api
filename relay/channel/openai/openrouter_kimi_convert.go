package openai

import (
	"github.com/QuantumNous/new-api/common"
)

func ConvertOpenRouterKimiResponse(responseBody []byte) ([]byte, error) {
	var body map[string]any
	if err := common.Unmarshal(responseBody, &body); err != nil {
		return responseBody, err
	}

	convertOpenRouterKimiBody(body)

	return common.Marshal(body)
}

func ConvertOpenRouterKimiStreamChunk(data string) string {
	var body map[string]any
	if err := common.Unmarshal(common.StringToByteSlice(data), &body); err != nil {
		return data
	}

	convertOpenRouterKimiBody(body)

	result, err := common.Marshal(body)
	if err != nil {
		return data
	}
	return string(result)
}

func convertOpenRouterKimiBody(body map[string]any) {
	delete(body, "provider")
	delete(body, "service_tier")
	delete(body, "system_fingerprint")

	if choices, ok := body["choices"].([]any); ok {
		for _, choice := range choices {
			choiceMap, ok := choice.(map[string]any)
			if !ok {
				continue
			}
			delete(choiceMap, "logprobs")
			delete(choiceMap, "native_finish_reason")

			if msg, ok := choiceMap["message"].(map[string]any); ok {
				convertKimiMessageFields(msg)
			}
			if delta, ok := choiceMap["delta"].(map[string]any); ok {
				convertKimiMessageFields(delta)
			}
		}
	}

	if usage, ok := body["usage"].(map[string]any); ok {
		delete(usage, "cost")
		delete(usage, "cost_details")
		delete(usage, "is_byok")

		var cachedTokens float64
		if ptd, ok := usage["prompt_tokens_details"].(map[string]any); ok {
			if ct, ok := ptd["cached_tokens"].(float64); ok {
				cachedTokens = ct
			}
			usage["prompt_tokens_details"] = map[string]any{
				"cached_tokens": cachedTokens,
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

		usage["cached_tokens"] = cachedTokens
	}
}

func convertKimiMessageFields(msg map[string]any) {
	if reasoning, exists := msg["reasoning"]; exists {
		msg["reasoning_content"] = reasoning
		delete(msg, "reasoning")
	}
	delete(msg, "reasoning_details")
	delete(msg, "refusal")
}
