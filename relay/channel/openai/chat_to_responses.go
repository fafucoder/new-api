package openai

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// responsesID returns a Responses-style id (resp_...) derived from the request id.
func responsesID(c *gin.Context) string {
	logID := c.GetString(common.RequestIdKey)
	if logID == "" {
		return "resp_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "resp_" + logID
}

// ResponsesIDFromContext returns a Responses-style id (resp_...) for the current request. It is
// exported for other channel packages (e.g. claude) that reuse the responses downgrade flow.
func ResponsesIDFromContext(c *gin.Context) string {
	return responsesID(c)
}

// OaiChatToResponsesHandler reads a non-stream Chat Completions upstream response and rewrites
// it into an OpenAI Responses API response for the client.
func OaiChatToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	respID := responsesID(c)
	responsesResp, err := service.ChatCompletionsResponseToResponsesResponse(&chatResp, respID)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	usage := &chatResp.Usage
	if usage.TotalTokens == 0 {
		text := ""
		if len(chatResp.Choices) > 0 {
			text = chatResp.Choices[0].Message.StringContent() + chatResp.Choices[0].Message.GetReasoningContent()
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

// chatToResponsesToolCallState tracks a single streaming tool call across chunks, keyed by index.
type chatToResponsesToolCallState struct {
	itemID   string
	callID   string
	name     string
	args     strings.Builder
	added    bool
	outIndex int
}

// OaiChatToResponsesStreamHandler consumes an upstream Chat Completions SSE stream and emits an
// OpenAI Responses API event stream to the client.
func OaiChatToResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	emitter := NewChatToResponsesEmitter(c, info, responsesID(c), time.Now().Unix(), info.UpstreamModelName)

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if emitter.Err() != nil {
			sr.Stop(emitter.Err())
			return
		}

		var chatChunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chatChunk); err != nil {
			logger.LogError(c, "failed to unmarshal chat stream chunk: "+err.Error())
			sr.Error(err)
			return
		}

		if !emitter.ProcessChatChunk(&chatChunk) {
			sr.Stop(emitter.Err())
			return
		}
	})

	if emitter.Err() != nil {
		return nil, emitter.Err()
	}

	usage := emitter.Finish()
	if emitter.Err() != nil {
		return nil, emitter.Err()
	}
	return usage, nil
}

