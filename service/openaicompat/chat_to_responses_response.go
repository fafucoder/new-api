package openaicompat

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// ChatCompletionsResponseToResponsesResponse converts a Chat Completions response into an
// OpenAI Responses API response. It is the inverse of ResponsesResponseToChatCompletionsResponse
// and is used when an upstream only supports /v1/chat/completions but the client calls /v1/responses.
func ChatCompletionsResponseToResponsesResponse(resp *dto.OpenAITextResponse, respID string) (*dto.OpenAIResponsesResponse, error) {
	if resp == nil {
		return nil, errors.New("response is nil")
	}

	out := &dto.OpenAIResponsesResponse{
		ID:        respID,
		Object:    "response",
		CreatedAt: interfaceToInt(resp.Created),
		Model:     resp.Model,
		Status:    json.RawMessage(`"completed"`),
		Output:    make([]dto.ResponsesOutput, 0),
	}

	status := "completed"
	itemIdx := 0

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		msg := choice.Message

		// reasoning content => reasoning output item
		if reasoning := msg.GetReasoningContent(); reasoning != "" {
			out.Output = append(out.Output, dto.ResponsesOutput{
				Type:   "reasoning",
				ID:     "rs_" + strconv.Itoa(itemIdx),
				Status: "completed",
				Content: []dto.ResponsesOutputContent{
					{Type: "reasoning_text", Text: reasoning},
				},
			})
			itemIdx++
		}

		// assistant text => message output item
		if text := msg.StringContent(); text != "" {
			out.Output = append(out.Output, dto.ResponsesOutput{
				Type:   "message",
				ID:     "msg_" + strconv.Itoa(itemIdx),
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: text},
				},
			})
			itemIdx++
		}

		// tool calls => function_call output items
		for _, tc := range msg.ParseToolCalls() {
			name := strings.TrimSpace(tc.Function.Name)
			if name == "" {
				continue
			}
			args := tc.Function.Arguments
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			out.Output = append(out.Output, dto.ResponsesOutput{
				Type:      "function_call",
				ID:        "fc_" + strconv.Itoa(itemIdx),
				Status:    "completed",
				CallId:    tc.ID,
				Name:      name,
				Arguments: json.RawMessage(args),
			})
			itemIdx++
		}

		if choice.FinishReason == "length" {
			status = "incomplete"
			out.IncompleteDetails = &dto.IncompleteDetails{Reasoning: "max_output_tokens"}
		}
	}

	out.Status = json.RawMessage(`"` + status + `"`)

	// usage mapping (inverse of ResponsesResponseToChatCompletionsResponse)
	usage := &dto.Usage{}
	usage.PromptTokens = resp.Usage.PromptTokens
	usage.CompletionTokens = resp.Usage.CompletionTokens
	usage.InputTokens = resp.Usage.PromptTokens
	usage.OutputTokens = resp.Usage.CompletionTokens
	if resp.Usage.TotalTokens != 0 {
		usage.TotalTokens = resp.Usage.TotalTokens
	} else {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	usage.InputTokensDetails = &dto.InputTokenDetails{
		CachedTokens: resp.Usage.PromptTokensDetails.CachedTokens,
		ImageTokens:  resp.Usage.PromptTokensDetails.ImageTokens,
		AudioTokens:  resp.Usage.PromptTokensDetails.AudioTokens,
	}
	usage.PromptTokensDetails = resp.Usage.PromptTokensDetails
	usage.CompletionTokenDetails.ReasoningTokens = resp.Usage.CompletionTokenDetails.ReasoningTokens
	out.Usage = usage

	return out, nil
}

func interfaceToInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return 0
}
