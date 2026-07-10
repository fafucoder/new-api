package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestResponsesRequestToChatCompletionsRequest_Basic(t *testing.T) {
	req := &dto.OpenAIResponsesRequest{
		Model:        "glm-5.2",
		Input:        []byte(`"hello"`),
		Instructions: []byte(`"you are helpful"`),
	}
	out, err := ResponsesRequestToChatCompletionsRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Model != "glm-5.2" {
		t.Errorf("model = %q, want glm-5.2", out.Model)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("messages = %d, want 2 (system + user)", len(out.Messages))
	}
	if out.Messages[0].Role != "system" || out.Messages[0].StringContent() != "you are helpful" {
		t.Errorf("system message wrong: %+v", out.Messages[0])
	}
	if out.Messages[1].Role != "user" || out.Messages[1].StringContent() != "hello" {
		t.Errorf("user message wrong: %+v", out.Messages[1])
	}
}

func TestResponsesRequestToChatCompletionsRequest_ToolCallRoundTrip(t *testing.T) {
	// input array: assistant function_call + function_call_output + tools
	req := &dto.OpenAIResponsesRequest{
		Model: "m",
		Input: []byte(`[
			{"role":"user","content":"weather?"},
			{"type":"function_call","call_id":"c1","name":"get_weather","arguments":"{\"city\":\"SF\"}"},
			{"type":"function_call_output","call_id":"c1","output":"sunny"}
		]`),
		Tools: []byte(`[{"type":"function","name":"get_weather","description":"d","parameters":{"type":"object"}}]`),
	}
	out, err := ResponsesRequestToChatCompletionsRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(out.Messages))
	}
	// assistant tool_calls
	tcs := out.Messages[1].ParseToolCalls()
	if len(tcs) != 1 || tcs[0].ID != "c1" || tcs[0].Function.Name != "get_weather" {
		t.Errorf("tool_calls wrong: %+v", tcs)
	}
	// tool message
	if out.Messages[2].Role != "tool" || out.Messages[2].ToolCallId != "c1" || out.Messages[2].StringContent() != "sunny" {
		t.Errorf("tool message wrong: %+v", out.Messages[2])
	}
	// tools converted to chat shape
	if len(out.Tools) != 1 || out.Tools[0].Type != "function" || out.Tools[0].Function.Name != "get_weather" {
		t.Errorf("tools wrong: %+v", out.Tools)
	}
}

func TestChatCompletionsResponseToResponsesResponse_TextAndUsage(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Model:  "glm-5.2",
		Object: "chat.completion",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				Message:      dto.Message{Role: "assistant", Content: "TCP is reliable."},
				FinishReason: "stop",
			},
		},
	}
	chat.Usage.PromptTokens = 22
	chat.Usage.CompletionTokens = 5
	chat.Usage.TotalTokens = 27
	chat.Usage.PromptTokensDetails.CachedTokens = 23

	out, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID != "resp_test" || out.Object != "response" {
		t.Errorf("id/object wrong: %s / %s", out.ID, out.Object)
	}
	if len(out.Output) != 1 || out.Output[0].Type != "message" {
		t.Fatalf("output wrong: %+v", out.Output)
	}
	if len(out.Output[0].Content) != 1 || out.Output[0].Content[0].Text != "TCP is reliable." {
		t.Errorf("output text wrong: %+v", out.Output[0].Content)
	}
	if out.Usage == nil || out.Usage.InputTokens != 22 || out.Usage.OutputTokens != 5 {
		t.Errorf("usage tokens wrong: %+v", out.Usage)
	}
	// cached_tokens must be preserved into input_tokens_details
	if out.Usage.InputTokensDetails == nil || out.Usage.InputTokensDetails.CachedTokens != 23 {
		t.Errorf("cached_tokens not preserved: %+v", out.Usage.InputTokensDetails)
	}
}

func TestChatCompletionsResponseToResponsesResponse_ToolCalls(t *testing.T) {
	msg := dto.Message{Role: "assistant"}
	msg.SetToolCalls([]dto.ToolCallResponse{
		{ID: "c1", Type: "function", Function: dto.FunctionResponse{Name: "get_weather", Arguments: `{"city":"SF"}`}},
	})
	chat := &dto.OpenAITextResponse{
		Model:   "m",
		Choices: []dto.OpenAITextResponseChoice{{Message: msg, FinishReason: "tool_calls"}},
	}
	out, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Output) != 1 || out.Output[0].Type != "function_call" {
		t.Fatalf("output wrong: %+v", out.Output)
	}
	if out.Output[0].CallId != "c1" || out.Output[0].Name != "get_weather" {
		t.Errorf("function_call fields wrong: %+v", out.Output[0])
	}
	if string(out.Output[0].Arguments) != `{"city":"SF"}` {
		t.Errorf("arguments wrong: %s", string(out.Output[0].Arguments))
	}
}
