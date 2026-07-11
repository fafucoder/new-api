package openai

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// ChatToResponsesEmitter turns a sequence of Chat Completions stream chunks into an OpenAI
// Responses API event stream. It is shared by the OpenAI and Claude downgrade paths: OpenAI feeds
// it chunks parsed from an upstream chat SSE stream, Claude feeds it chunks produced by
// StreamResponseClaude2OpenAI.
type ChatToResponsesEmitter struct {
	c    *gin.Context
	info *relaycommon.RelayInfo

	respID    string
	createdAt int64
	model     string
	lockModel bool // when true, the initial model is authoritative and chunk models are ignored

	usage      *dto.Usage
	outputText strings.Builder
	usageText  strings.Builder
	reasoning  strings.Builder

	sentCreated bool
	streamErr   *types.NewAPIError

	nextOutputIndex   int
	msgItemID         string
	msgAdded          bool
	msgOutIndex       int
	reasoningItemID   string
	reasoningAdded    bool
	reasoningOutIndex int

	toolStates map[int]*chatToResponsesToolCallState
	toolOrder  []int
}

// NewChatToResponsesEmitter creates an emitter. respID/createdAt/model provide the initial
// Responses envelope values; they may be updated from incoming chunks.
func NewChatToResponsesEmitter(c *gin.Context, info *relaycommon.RelayInfo, respID string, createdAt int64, model string) *ChatToResponsesEmitter {
	lockModel := false
	if unified := info.GetClientFacingModelName(); unified != "" {
		// 渠道开启统一模型名：锁定为请求名，忽略上游 chunk 里的 model
		model = unified
		lockModel = true
	}
	return &ChatToResponsesEmitter{
		c:          c,
		info:       info,
		respID:     respID,
		createdAt:  createdAt,
		model:      model,
		lockModel:  lockModel,
		usage:      &dto.Usage{},
		toolStates: make(map[int]*chatToResponsesToolCallState),
	}
}

// Err returns any terminal error encountered while emitting events.
func (e *ChatToResponsesEmitter) Err() *types.NewAPIError { return e.streamErr }

// SetUsage overrides the accumulated usage (used by the Claude path, whose usage is not carried on
// the converted chat chunks).
func (e *ChatToResponsesEmitter) SetUsage(usage *dto.Usage) {
	if usage != nil {
		e.usage = usage
	}
}

func (e *ChatToResponsesEmitter) ptr(i int) *int { return &i }

func (e *ChatToResponsesEmitter) send(streamResp dto.ResponsesStreamResponse) bool {
	data, err := common.Marshal(streamResp)
	if err != nil {
		e.streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		return false
	}
	helper.ResponseChunkData(e.c, streamResp, string(data))
	return true
}

func (e *ChatToResponsesEmitter) sendCreatedIfNeeded() bool {
	if e.sentCreated {
		return true
	}
	created := &dto.OpenAIResponsesResponse{
		ID:        e.respID,
		Object:    "response",
		CreatedAt: int(e.createdAt),
		Model:     e.model,
		Status:    []byte(`"in_progress"`),
		Output:    []dto.ResponsesOutput{},
	}
	if !e.send(dto.ResponsesStreamResponse{Type: "response.created", Response: created}) {
		return false
	}
	if !e.send(dto.ResponsesStreamResponse{Type: "response.in_progress", Response: created}) {
		return false
	}
	e.sentCreated = true
	return true
}

func (e *ChatToResponsesEmitter) ensureReasoningItem() bool {
	if e.reasoningAdded {
		return true
	}
	e.reasoningItemID = "rs_" + strconv.Itoa(e.nextOutputIndex)
	e.reasoningOutIndex = e.nextOutputIndex
	e.nextOutputIndex++
	item := &dto.ResponsesOutput{Type: "reasoning", ID: e.reasoningItemID, Status: "in_progress"}
	if !e.send(dto.ResponsesStreamResponse{Type: "response.output_item.added", OutputIndex: e.ptr(e.reasoningOutIndex), Item: item}) {
		return false
	}
	e.reasoningAdded = true
	return true
}

func (e *ChatToResponsesEmitter) ensureMessageItem() bool {
	if e.msgAdded {
		return true
	}
	e.msgItemID = "msg_" + strconv.Itoa(e.nextOutputIndex)
	e.msgOutIndex = e.nextOutputIndex
	e.nextOutputIndex++
	item := &dto.ResponsesOutput{Type: "message", ID: e.msgItemID, Status: "in_progress", Role: "assistant"}
	if !e.send(dto.ResponsesStreamResponse{Type: "response.output_item.added", OutputIndex: e.ptr(e.msgOutIndex), Item: item}) {
		return false
	}
	e.msgAdded = true
	return true
}

func (e *ChatToResponsesEmitter) handleTextDelta(delta string) bool {
	if delta == "" {
		return true
	}
	if !e.sendCreatedIfNeeded() || !e.ensureMessageItem() {
		return false
	}
	e.outputText.WriteString(delta)
	e.usageText.WriteString(delta)
	return e.send(dto.ResponsesStreamResponse{
		Type:         "response.output_text.delta",
		ItemID:       e.msgItemID,
		OutputIndex:  e.ptr(e.msgOutIndex),
		ContentIndex: e.ptr(0),
		Delta:        delta,
	})
}

func (e *ChatToResponsesEmitter) handleReasoningDelta(delta string) bool {
	if delta == "" {
		return true
	}
	if !e.sendCreatedIfNeeded() || !e.ensureReasoningItem() {
		return false
	}
	e.reasoning.WriteString(delta)
	e.usageText.WriteString(delta)
	return e.send(dto.ResponsesStreamResponse{
		Type:         "response.reasoning_summary_text.delta",
		ItemID:       e.reasoningItemID,
		OutputIndex:  e.ptr(e.reasoningOutIndex),
		SummaryIndex: e.ptr(0),
		Delta:        delta,
	})
}

func (e *ChatToResponsesEmitter) handleToolCall(tc dto.ToolCallResponse) bool {
	idx := 0
	if tc.Index != nil {
		idx = *tc.Index
	}
	state, ok := e.toolStates[idx]
	if !ok {
		state = &chatToResponsesToolCallState{}
		e.toolStates[idx] = state
		e.toolOrder = append(e.toolOrder, idx)
	}
	if tc.ID != "" {
		state.callID = tc.ID
	}
	if tc.Function.Name != "" {
		state.name = tc.Function.Name
	}

	if !state.added {
		if !e.sendCreatedIfNeeded() {
			return false
		}
		state.outIndex = e.nextOutputIndex
		e.nextOutputIndex++
		state.itemID = "fc_" + strconv.Itoa(state.outIndex)
		callID := state.callID
		if callID == "" {
			callID = state.itemID
		}
		item := &dto.ResponsesOutput{
			Type:   "function_call",
			ID:     state.itemID,
			Status: "in_progress",
			CallId: callID,
			Name:   state.name,
		}
		if !e.send(dto.ResponsesStreamResponse{Type: "response.output_item.added", OutputIndex: e.ptr(state.outIndex), Item: item}) {
			return false
		}
		state.added = true
		e.usageText.WriteString(state.name)
	}

	if tc.Function.Arguments != "" {
		state.args.WriteString(tc.Function.Arguments)
		e.usageText.WriteString(tc.Function.Arguments)
		if !e.send(dto.ResponsesStreamResponse{
			Type:        "response.function_call_arguments.delta",
			ItemID:      state.itemID,
			OutputIndex: e.ptr(state.outIndex),
			Delta:       tc.Function.Arguments,
		}) {
			return false
		}
	}
	return true
}

// ProcessChatChunk emits Responses events for a single Chat Completions stream chunk. Returns false
// if a terminal error occurred (check Err()).
func (e *ChatToResponsesEmitter) ProcessChatChunk(chatChunk *dto.ChatCompletionsStreamResponse) bool {
	if chatChunk == nil {
		return e.streamErr == nil
	}
	if chatChunk.Model != "" && !e.lockModel {
		e.model = chatChunk.Model
	}
	if chatChunk.Created != 0 {
		e.createdAt = chatChunk.Created
	}
	if chatChunk.Usage != nil && chatChunk.Usage.TotalTokens != 0 {
		e.usage = chatChunk.Usage
	}

	if len(chatChunk.Choices) == 0 {
		return e.streamErr == nil
	}
	choice := chatChunk.Choices[0]

	if r := choice.Delta.GetReasoningContent(); r != "" {
		if !e.handleReasoningDelta(r) {
			return false
		}
	}
	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		if !e.handleTextDelta(*choice.Delta.Content) {
			return false
		}
	}
	for _, tc := range choice.Delta.ToolCalls {
		if !e.handleToolCall(tc) {
			return false
		}
	}
	return true
}

// Finish emits the terminal done/completed events and returns the final usage. Any error is
// available via Err().
func (e *ChatToResponsesEmitter) Finish() *dto.Usage {
	if e.streamErr != nil {
		return nil
	}
	if !e.sendCreatedIfNeeded() {
		return nil
	}

	finalOutput := make([]dto.ResponsesOutput, 0, e.nextOutputIndex)

	if e.reasoningAdded {
		reasoningText := e.reasoning.String()
		if !e.send(dto.ResponsesStreamResponse{
			Type:         "response.reasoning_summary_text.done",
			ItemID:       e.reasoningItemID,
			OutputIndex:  e.ptr(e.reasoningOutIndex),
			SummaryIndex: e.ptr(0),
		}) {
			return nil
		}
		reasoningItem := dto.ResponsesOutput{
			Type: "reasoning", ID: e.reasoningItemID, Status: "completed",
			Content: []dto.ResponsesOutputContent{{Type: "reasoning_text", Text: reasoningText}},
		}
		if !e.send(dto.ResponsesStreamResponse{Type: "response.output_item.done", OutputIndex: e.ptr(e.reasoningOutIndex), Item: &reasoningItem}) {
			return nil
		}
		finalOutput = append(finalOutput, reasoningItem)
	}

	if e.msgAdded {
		text := e.outputText.String()
		if !e.send(dto.ResponsesStreamResponse{
			Type: "response.output_text.done", ItemID: e.msgItemID, OutputIndex: e.ptr(e.msgOutIndex), ContentIndex: e.ptr(0),
		}) {
			return nil
		}
		msgItem := dto.ResponsesOutput{
			Type: "message", ID: e.msgItemID, Status: "completed", Role: "assistant",
			Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: text}},
		}
		if !e.send(dto.ResponsesStreamResponse{Type: "response.output_item.done", OutputIndex: e.ptr(e.msgOutIndex), Item: &msgItem}) {
			return nil
		}
		finalOutput = append(finalOutput, msgItem)
	}

	for _, idx := range e.toolOrder {
		state := e.toolStates[idx]
		if state == nil || !state.added {
			continue
		}
		args := state.args.String()
		if !e.send(dto.ResponsesStreamResponse{
			Type: "response.function_call_arguments.done", ItemID: state.itemID, OutputIndex: e.ptr(state.outIndex), Delta: args,
		}) {
			return nil
		}
		callID := state.callID
		if callID == "" {
			callID = state.itemID
		}
		if strings.TrimSpace(args) == "" {
			args = "{}"
		}
		fcItem := dto.ResponsesOutput{
			Type: "function_call", ID: state.itemID, Status: "completed",
			CallId: callID, Name: state.name, Arguments: []byte(args),
		}
		if !e.send(dto.ResponsesStreamResponse{Type: "response.output_item.done", OutputIndex: e.ptr(state.outIndex), Item: &fcItem}) {
			return nil
		}
		finalOutput = append(finalOutput, fcItem)
	}

	if e.usage == nil || e.usage.TotalTokens == 0 {
		e.usage = service.ResponseText2Usage(e.c, e.usageText.String(), e.info.UpstreamModelName, e.info.GetEstimatePromptTokens())
	}

	completed := &dto.OpenAIResponsesResponse{
		ID:        e.respID,
		Object:    "response",
		CreatedAt: int(e.createdAt),
		Model:     e.model,
		Status:    []byte(`"completed"`),
		Output:    finalOutput,
		Usage:     e.usage,
	}
	if !e.send(dto.ResponsesStreamResponse{Type: "response.completed", Response: completed}) {
		return nil
	}

	return e.usage
}
