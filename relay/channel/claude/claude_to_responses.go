package claude

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// claudeUsageToOpenAIUsage maps a Claude usage block into the OpenAI-style dto.Usage, preserving
// cache-read tokens as cached prompt tokens.
func claudeUsageToOpenAIUsage(cu *dto.ClaudeUsage, usage *dto.Usage) {
	if cu == nil || usage == nil {
		return
	}
	if cu.InputTokens != 0 {
		usage.PromptTokens = cu.InputTokens
		usage.InputTokens = cu.InputTokens
	}
	if cu.OutputTokens != 0 {
		usage.CompletionTokens = cu.OutputTokens
		usage.OutputTokens = cu.OutputTokens
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	usage.UsageSemantic = "anthropic"
	if cu.CacheReadInputTokens != 0 {
		usage.PromptTokensDetails.CachedTokens = cu.CacheReadInputTokens
	}
	if cu.CacheCreationInputTokens != 0 {
		usage.PromptTokensDetails.CachedCreationTokens = cu.CacheCreationInputTokens
	}
}

// ClaudeToResponsesHandler reads a non-stream Claude Messages upstream response and rewrites it into
// an OpenAI Responses API response for the client. Used by the /v1/responses -> /v1/messages
// downgrade path.
func ClaudeToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var claudeResp dto.ClaudeResponse
	if err := common.Unmarshal(body, &claudeResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if claudeError := claudeResp.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return nil, types.WithClaudeError(*claudeError, resp.StatusCode)
	}

	openaiResp := ResponseClaude2OpenAI(&claudeResp)
	usage := &dto.Usage{}
	claudeUsageToOpenAIUsage(claudeResp.Usage, usage)
	openaiResp.Usage = *usage

	respID := openaichannel.ResponsesIDFromContext(c)
	responsesResp, err := service.ChatCompletionsResponseToResponsesResponse(openaiResp, respID)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if usage.TotalTokens == 0 {
		text := ""
		if len(openaiResp.Choices) > 0 {
			text = openaiResp.Choices[0].Message.StringContent() + openaiResp.Choices[0].Message.GetReasoningContent()
		}
		usage = service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
		responsesResp.Usage = usage
	}

	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

// ClaudeToResponsesStreamHandler consumes an upstream Claude Messages SSE stream and emits an OpenAI
// Responses API event stream to the client, reusing the shared chat->responses emitter.
func ClaudeToResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	emitter := openaichannel.NewChatToResponsesEmitter(c, info, openaichannel.ResponsesIDFromContext(c), time.Now().Unix(), info.UpstreamModelName)
	usage := &dto.Usage{}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if emitter.Err() != nil {
			sr.Stop(emitter.Err())
			return
		}

		var claudeResponse dto.ClaudeResponse
		if err := common.UnmarshalJsonStr(data, &claudeResponse); err != nil {
			logger.LogError(c, "failed to unmarshal claude stream event: "+err.Error())
			sr.Error(err)
			return
		}
		if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
			sr.Stop(types.WithClaudeError(*claudeError, http.StatusInternalServerError))
			return
		}

		// accumulate usage from message_start / message_delta events
		if claudeResponse.Message != nil && claudeResponse.Message.Usage != nil {
			claudeUsageToOpenAIUsage(claudeResponse.Message.Usage, usage)
		}
		if claudeResponse.Usage != nil {
			claudeUsageToOpenAIUsage(claudeResponse.Usage, usage)
		}

		chatChunk := StreamResponseClaude2OpenAI(&claudeResponse)
		if chatChunk == nil {
			return
		}
		if !emitter.ProcessChatChunk(chatChunk) {
			sr.Stop(emitter.Err())
			return
		}
	})

	if emitter.Err() != nil {
		return nil, emitter.Err()
	}

	if usage.TotalTokens != 0 {
		emitter.SetUsage(usage)
	}

	finalUsage := emitter.Finish()
	if emitter.Err() != nil {
		return nil, emitter.Err()
	}
	return finalUsage, nil
}
