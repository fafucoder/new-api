package relay

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"

	"github.com/gin-gonic/gin"
)

// buildContentFilterBypassUsage 为被吞掉的 content_filter 请求构造计费用量。
// 上游 400 时通常没有可用的 usage，这里按预估的 prompt token 计费，
// completion 记为 0（返回给客户端的是空内容）。
func buildContentFilterBypassUsage(info *relaycommon.RelayInfo) *dto.Usage {
	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens < 0 {
		promptTokens = 0
	}
	return &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}
}

// writeContentFilterBypassResponse 将被吞掉的 content_filter 请求伪装成一次正常完成，
// 返回空内容、finish_reason=stop。流式 / 非流式分别处理。
func writeContentFilterBypassResponse(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) {
	responseId := helper.GetResponseID(c)
	createAt := time.Now().Unix()

	model := info.GetClientFacingModelName()
	if model == "" {
		model = info.UpstreamModelName
	}

	if info.IsStream {
		writeContentFilterBypassStream(c, info, usage, responseId, createAt, model)
		return
	}
	writeContentFilterBypassJSON(c, usage, responseId, createAt, model)
}

func writeContentFilterBypassJSON(c *gin.Context, usage *dto.Usage, responseId string, createAt int64, model string) {
	response := dto.OpenAITextResponse{
		Id:      responseId,
		Object:  "chat.completion",
		Created: createAt,
		Model:   model,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index:        0,
				FinishReason: constant.FinishReasonStop,
			},
		},
	}
	response.Choices[0].Message.Role = "assistant"
	response.Choices[0].Message.SetStringContent("")
	if usage != nil {
		response.Usage = *usage
	}
	c.JSON(http.StatusOK, response)
}

func writeContentFilterBypassStream(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage, responseId string, createAt int64, model string) {
	helper.SetEventStreamHeaders(c)

	emptyContent := ""
	deltaChunk := &dto.ChatCompletionsStreamResponse{
		Id:      responseId,
		Object:  "chat.completion.chunk",
		Created: createAt,
		Model:   model,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: &emptyContent,
				},
			},
		},
	}
	_ = helper.ObjectData(c, deltaChunk)
	info.SetFirstResponseTime()

	stopChunk := helper.GenerateStopResponse(responseId, createAt, model, constant.FinishReasonStop)
	_ = helper.ObjectData(c, stopChunk)

	if info.ShouldIncludeUsage && usage != nil {
		usageChunk := helper.GenerateFinalUsageResponse(responseId, createAt, model, *usage)
		_ = helper.ObjectData(c, usageChunk)
	}

	helper.Done(c)
}
