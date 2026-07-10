package openaicompat

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

// ResponsesRequestToChatCompletionsRequest converts an OpenAI Responses API request into a
// Chat Completions request. It is the inverse of ChatCompletionsRequestToResponsesRequest and
// is used when an upstream only supports /v1/chat/completions but the client calls /v1/responses.
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	messages := make([]dto.Message, 0)

	// instructions => leading system message
	if len(req.Instructions) > 0 {
		var instructions string
		if err := common.Unmarshal(req.Instructions, &instructions); err == nil {
			if s := strings.TrimSpace(instructions); s != "" {
				messages = append(messages, dto.Message{Role: "system", Content: s})
			}
		}
	}

	inputMessages, err := responsesInputToMessages(req.Input)
	if err != nil {
		return nil, err
	}
	messages = append(messages, inputMessages...)

	out := &dto.GeneralOpenAIRequest{
		Model:             req.Model,
		Messages:          messages,
		Stream:            req.Stream,
		Temperature:       req.Temperature,
		TopP:              req.TopP,
		User:              req.User,
		Store:             req.Store,
		Metadata:          req.Metadata,
		StreamOptions:     req.StreamOptions,
		MaxCompletionTokens: req.MaxOutputTokens,
	}

	if req.ParallelToolCalls != nil {
		var parallel bool
		if err := common.Unmarshal(req.ParallelToolCalls, &parallel); err == nil {
			out.ParallelTooCalls = lo.ToPtr(parallel)
		}
	}

	if tools := responsesToolsToChatTools(req); len(tools) > 0 {
		out.Tools = tools
	}

	if tc := responsesToolChoiceToChatToolChoice(req.ToolChoice); tc != nil {
		out.ToolChoice = tc
	}

	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		out.ReasoningEffort = req.Reasoning.Effort
	}

	if rf := responsesTextToChatResponseFormat(req.Text); rf != nil {
		out.ResponseFormat = rf
	}

	return out, nil
}

// responsesInputToMessages parses the Responses `input` field (string or array) into chat messages.
func responsesInputToMessages(input json.RawMessage) ([]dto.Message, error) {
	if len(input) == 0 {
		return nil, nil
	}

	switch common.GetJsonType(input) {
	case "string":
		var s string
		if err := common.Unmarshal(input, &s); err != nil {
			return nil, err
		}
		return []dto.Message{{Role: "user", Content: s}}, nil
	case "array":
		var items []map[string]any
		if err := common.Unmarshal(input, &items); err != nil {
			return nil, err
		}
		return responsesInputItemsToMessages(items), nil
	default:
		return nil, errors.New("unsupported responses input type")
	}
}

func responsesInputItemsToMessages(items []map[string]any) []dto.Message {
	messages := make([]dto.Message, 0, len(items))

	for _, item := range items {
		itemType := common.Interface2String(item["type"])

		switch itemType {
		case "function_call":
			// assistant tool call
			callID := common.Interface2String(item["call_id"])
			name := common.Interface2String(item["name"])
			if callID == "" || name == "" {
				continue
			}
			args := responsesArgumentsToString(item["arguments"])
			toolCall := dto.ToolCallResponse{
				ID:   callID,
				Type: "function",
				Function: dto.FunctionResponse{
					Name:      name,
					Arguments: args,
				},
			}
			msg := dto.Message{Role: "assistant"}
			msg.SetToolCalls([]dto.ToolCallResponse{toolCall})
			messages = append(messages, msg)
			continue
		case "function_call_output":
			callID := common.Interface2String(item["call_id"])
			if callID == "" {
				continue
			}
			var output string
			switch v := item["output"].(type) {
			case string:
				output = v
			case nil:
				output = ""
			default:
				if b, err := common.Marshal(v); err == nil {
					output = string(b)
				}
			}
			messages = append(messages, dto.Message{
				Role:       "tool",
				ToolCallId: callID,
				Content:    output,
			})
			continue
		}

		// role-based message (user/assistant/system/developer)
		role := strings.TrimSpace(common.Interface2String(item["role"]))
		if role == "" {
			continue
		}
		if role == "developer" {
			role = "system"
		}

		content, ok := item["content"]
		if !ok || content == nil {
			messages = append(messages, dto.Message{Role: role, Content: ""})
			continue
		}

		switch cv := content.(type) {
		case string:
			messages = append(messages, dto.Message{Role: role, Content: cv})
		case []any:
			media := responsesContentPartsToMedia(cv)
			msg := dto.Message{Role: role}
			msg.SetMediaContent(media)
			messages = append(messages, msg)
		default:
			messages = append(messages, dto.Message{Role: role, Content: ""})
		}
	}

	return messages
}

// responsesContentPartsToMedia converts Responses content parts back into chat MediaContent.
func responsesContentPartsToMedia(parts []any) []dto.MediaContent {
	media := make([]dto.MediaContent, 0, len(parts))
	for _, p := range parts {
		part, ok := p.(map[string]any)
		if !ok {
			continue
		}
		switch common.Interface2String(part["type"]) {
		case "input_text", "output_text", "text":
			media = append(media, dto.MediaContent{
				Type: dto.ContentTypeText,
				Text: common.Interface2String(part["text"]),
			})
		case "input_image":
			// image_url in responses is a plain string; chat expects an object {url:...}
			imageURL := part["image_url"]
			media = append(media, dto.MediaContent{
				Type:     dto.ContentTypeImageURL,
				ImageUrl: map[string]any{"url": common.Interface2String(imageURL)},
			})
		case "input_file":
			media = append(media, dto.MediaContent{
				Type: dto.ContentTypeFile,
				File: part["file"],
			})
		case "input_audio":
			media = append(media, dto.MediaContent{
				Type:       dto.ContentTypeInputAudio,
				InputAudio: part["input_audio"],
			})
		case "input_video":
			media = append(media, dto.MediaContent{
				Type:     dto.ContentTypeVideoUrl,
				VideoUrl: part["video_url"],
			})
		default:
			// best effort: keep text if present
			if text := common.Interface2String(part["text"]); text != "" {
				media = append(media, dto.MediaContent{Type: dto.ContentTypeText, Text: text})
			}
		}
	}
	return media
}

// responsesToolsToChatTools converts Responses function tools to chat tools.
// Non-function (built-in) tools such as web_search_preview are dropped, as chat/completions
// upstreams cannot consume them.
func responsesToolsToChatTools(req *dto.OpenAIResponsesRequest) []dto.ToolCallRequest {
	toolsMap := req.GetToolsMap()
	if len(toolsMap) == 0 {
		return nil
	}
	tools := make([]dto.ToolCallRequest, 0, len(toolsMap))
	for _, tool := range toolsMap {
		if common.Interface2String(tool["type"]) != "function" {
			continue
		}
		tools = append(tools, dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        common.Interface2String(tool["name"]),
				Description: common.Interface2String(tool["description"]),
				Parameters:  tool["parameters"],
			},
		})
	}
	return tools
}

// responsesToolChoiceToChatToolChoice converts Responses tool_choice into chat tool_choice.
// Responses: {"type":"function","name":"x"}  Chat: {"type":"function","function":{"name":"x"}}
func responsesToolChoiceToChatToolChoice(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	switch common.GetJsonType(raw) {
	case "string":
		var s string
		if err := common.Unmarshal(raw, &s); err == nil {
			return s
		}
		return nil
	case "object":
		var m map[string]any
		if err := common.Unmarshal(raw, &m); err != nil {
			return nil
		}
		if common.Interface2String(m["type"]) == "function" {
			if name := common.Interface2String(m["name"]); name != "" {
				return map[string]any{
					"type":     "function",
					"function": map[string]any{"name": name},
				}
			}
		}
		return m
	default:
		return nil
	}
}

// responsesTextToChatResponseFormat converts Responses text.format into chat response_format.
func responsesTextToChatResponseFormat(raw json.RawMessage) *dto.ResponseFormat {
	if len(raw) == 0 {
		return nil
	}
	var textCfg struct {
		Format map[string]any `json:"format"`
	}
	if err := common.Unmarshal(raw, &textCfg); err != nil || len(textCfg.Format) == 0 {
		return nil
	}
	formatType := common.Interface2String(textCfg.Format["type"])
	if formatType == "" {
		return nil
	}
	rf := &dto.ResponseFormat{Type: formatType}
	if formatType == "json_schema" {
		schema := make(map[string]any)
		for k, v := range textCfg.Format {
			if k == "type" {
				continue
			}
			schema[k] = v
		}
		if len(schema) > 0 {
			if b, err := common.Marshal(schema); err == nil {
				rf.JsonSchema = b
			}
		}
	}
	return rf
}

func responsesArgumentsToString(arguments any) string {
	if arguments == nil {
		return ""
	}
	if s, ok := arguments.(string); ok {
		return s
	}
	if b, err := common.Marshal(arguments); err == nil {
		return string(b)
	}
	return ""
}
