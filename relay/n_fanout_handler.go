package relay

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// shouldNFanout 判断本次请求是否需要走网关侧 n fan-out：
// 渠道开启 NFanoutEnabled 开关，且客户端显式请求 n>1。
func shouldNFanout(info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) bool {
	if info == nil || request == nil {
		return false
	}
	if !info.ChannelSetting.NFanoutEnabled {
		return false
	}
	return request.N != nil && *request.N > 1
}

// nFanoutResult 单个子请求的结果
type nFanoutResult struct {
	resp *http.Response
	err  error
}

// nFanoutHelper 将一个 n>1 的请求拆成 n 份 n=1 的并发上游请求，
// 再把结果合并为包含 n 条 choices 的响应返回。用量按 n 份累加。
//
// jsonData 是已经过转换/裁剪/参数覆盖、且 n 已被置为 1 的上游请求体字节。
// 由于 n 份子请求请求体基本一致，这里对同一份字节做 n 次并发发送即可
// （非贪婪采样下依靠 temperature 等参数产生多样性）。
//
// 特殊处理 seed：若客户端显式传了 seed，则每份子请求使用 seed+i，
// 避免所有子请求复现到同一确定性输出、退化成 N 份相同结果，
// 同时整批仍可复现（同输入 → 同 N 条输出），保持与 OpenAI 原生 n 的多样性语义一致。
func nFanoutHelper(c *gin.Context, info *relaycommon.RelayInfo, adaptor channel.Adaptor, jsonData []byte, n int, isStream bool) (*dto.Usage, *types.NewAPIError) {
	// 读取客户端 seed（若存在），用于按份偏移
	seedResult := gjson.GetBytes(jsonData, "seed")
	hasSeed := seedResult.Exists()
	baseSeed := seedResult.Int()

	results := make([]nFanoutResult, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		idx := i
		// 每个子请求使用独立的请求体 buffer，避免共享 Reader 被并发消费
		bodyBytes := make([]byte, len(jsonData))
		copy(bodyBytes, jsonData)
		// seed 偏移：第 i 份用 baseSeed+i，恢复子请求间的多样性且整批可复现
		if hasSeed {
			if newBody, err := sjson.SetBytes(bodyBytes, "seed", baseSeed+int64(idx)); err == nil {
				bodyBytes = newBody
			} else {
				logger.LogError(c, fmt.Sprintf("n fanout: set seed for sub-request #%d failed: %s", idx, err.Error()))
			}
		}
		gopool.Go(func() {
			defer wg.Done()
			resp, err := adaptor.DoRequest(c, info, bytes.NewBuffer(bodyBytes))
			if err != nil {
				results[idx] = nFanoutResult{err: fmt.Errorf("fanout sub-request #%d failed: %w", idx, err)}
				return
			}
			httpResp, ok := resp.(*http.Response)
			if !ok || httpResp == nil {
				results[idx] = nFanoutResult{err: fmt.Errorf("fanout sub-request #%d got invalid response", idx)}
				return
			}
			results[idx] = nFanoutResult{resp: httpResp}
		})
	}
	wg.Wait()

	// 收集成功的上游响应，过滤失败/非 200
	statusCodeMappingStr := c.GetString("status_code_mapping")
	var okResps []*http.Response
	var firstErr *types.NewAPIError
	failed := 0
	for i := 0; i < n; i++ {
		r := results[i]
		if r.err != nil {
			failed++
			logger.LogError(c, r.err.Error())
			continue
		}
		if r.resp.StatusCode != http.StatusOK {
			failed++
			newApiErr := service.RelayErrorHandler(c.Request.Context(), r.resp, false)
			service.ResetStatusCode(newApiErr, statusCodeMappingStr)
			if firstErr == nil {
				firstErr = newApiErr
			} else {
				service.CloseResponseBodyGracefully(r.resp)
			}
			continue
		}
		okResps = append(okResps, r.resp)
	}

	if len(okResps) == 0 {
		// 全部失败：返回第一个上游错误，或通用错误
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, types.NewError(fmt.Errorf("all %d fanout sub-requests failed", n), types.ErrorCodeDoRequestFailed)
	}

	if failed > 0 {
		logger.LogWarn(c, fmt.Sprintf("n fanout: %d/%d sub-requests failed, returning %d choices", failed, n, len(okResps)))
	}

	if isStream {
		return nFanoutStreamMerge(c, info, okResps)
	}
	return nFanoutNonStreamMerge(c, info, okResps)
}

// nFanoutNonStreamMerge 合并 n 份非流式响应为单条含 n 个 choices 的响应
func nFanoutNonStreamMerge(c *gin.Context, info *relaycommon.RelayInfo, resps []*http.Response) (*dto.Usage, *types.NewAPIError) {
	unifyModel := info.GetClientFacingModelName()

	var merged dto.OpenAITextResponse
	totalUsage := &dto.Usage{}
	choiceIndex := 0

	for i, resp := range resps {
		body, err := io.ReadAll(resp.Body)
		service.CloseResponseBodyGracefully(resp)
		if err != nil {
			logger.LogError(c, fmt.Sprintf("n fanout: read sub-response #%d body failed: %s", i, err.Error()))
			continue
		}
		var subResp dto.OpenAITextResponse
		if err := common.Unmarshal(body, &subResp); err != nil {
			logger.LogError(c, fmt.Sprintf("n fanout: unmarshal sub-response #%d failed: %s", i, err.Error()))
			continue
		}
		if oaiErr := subResp.GetOpenAIError(); oaiErr != nil && oaiErr.Type != "" {
			logger.LogError(c, fmt.Sprintf("n fanout: sub-response #%d returned error: %s", i, oaiErr.Message))
			continue
		}

		if merged.Id == "" {
			merged.Id = subResp.Id
			merged.Object = subResp.Object
			merged.Created = subResp.Created
			merged.Model = subResp.Model
		}

		for _, choice := range subResp.Choices {
			choice.Index = choiceIndex
			choiceIndex++
			merged.Choices = append(merged.Choices, choice)
		}

		accumulateUsage(totalUsage, &subResp.Usage)
	}

	if len(merged.Choices) == 0 {
		return nil, types.NewError(fmt.Errorf("n fanout: no valid choices merged"), types.ErrorCodeBadResponseBody)
	}

	if unifyModel != "" {
		merged.Model = unifyModel
	}
	merged.Usage = *totalUsage

	respBody, err := common.Marshal(merged)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	service.IOCopyBytesGracefully(c, nil, respBody)

	return totalUsage, nil
}

// nFanoutStreamMerge 并发消费 n 份流式响应，按 slot 重映射 choice.Index 后交错回传 SSE，
// 累加各路 usage，最后统一发送 usage chunk（如需要）与 [DONE]。
func nFanoutStreamMerge(c *gin.Context, info *relaycommon.RelayInfo, resps []*http.Response) (*dto.Usage, *types.NewAPIError) {
	helper.SetEventStreamHeaders(c)
	info.IsStream = true

	unifyModel := info.GetClientFacingModelName()
	responseID := helper.GetResponseID(c)

	var writeMu sync.Mutex // 串行化对客户端的写入
	var usageMu sync.Mutex // 保护 usage 累加
	totalUsage := &dto.Usage{}

	var wg sync.WaitGroup
	for slot, resp := range resps {
		wg.Add(1)
		slotIdx := slot
		r := resp
		gopool.Go(func() {
			defer wg.Done()
			defer service.CloseResponseBodyGracefully(r)

			scanner := bufio.NewScanner(r.Body)
			scanner.Buffer(make([]byte, 1024*64), 1024*1024*16)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" {
					continue
				}
				if data == "[DONE]" {
					// 子流结束标记不转发，由主流程统一发送
					return
				}

				var chunk dto.ChatCompletionsStreamResponse
				if err := common.Unmarshal([]byte(data), &chunk); err != nil {
					logger.LogError(c, fmt.Sprintf("n fanout: unmarshal stream chunk (slot %d) failed: %s", slotIdx, err.Error()))
					continue
				}

				// 累加并剥离子流 usage，避免把单份 usage 透传给客户端
				if chunk.Usage != nil {
					usageMu.Lock()
					accumulateUsage(totalUsage, chunk.Usage)
					usageMu.Unlock()
					chunk.Usage = nil
					if len(chunk.Choices) == 0 {
						// 纯 usage chunk，不转发
						continue
					}
				}

				// 将子流 choice.Index 重映射到该 slot，避免多路 index 冲突
				for i := range chunk.Choices {
					chunk.Choices[i].Index = slotIdx
				}
				chunk.Id = responseID
				if unifyModel != "" {
					chunk.Model = unifyModel
				}

				writeMu.Lock()
				_ = helper.ObjectData(c, chunk)
				info.SetFirstResponseTime()
				writeMu.Unlock()
			}
			if err := scanner.Err(); err != nil {
				logger.LogError(c, fmt.Sprintf("n fanout: scan stream (slot %d) error: %s", slotIdx, err.Error()))
			}
		})
	}
	wg.Wait()

	// 若无上游 usage，则按累计文本估算不可行（此处已累加各路 usage）；
	// 补全 prompt 估算，避免 0 值
	if totalUsage.PromptTokens == 0 && totalUsage.CompletionTokens == 0 {
		totalUsage.PromptTokens = info.GetEstimatePromptTokens() * len(resps)
	}
	totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens

	// 统一发送 usage chunk（若客户端要求）与 [DONE]
	if info.ShouldIncludeUsage {
		usageChunk := &dto.ChatCompletionsStreamResponse{
			Id:      responseID,
			Object:  "chat.completion.chunk",
			Model:   info.UpstreamModelName,
			Choices: []dto.ChatCompletionsStreamResponseChoice{},
			Usage:   totalUsage,
		}
		if unifyModel != "" {
			usageChunk.Model = unifyModel
		}
		_ = helper.ObjectData(c, usageChunk)
	}
	helper.Done(c)

	return totalUsage, nil
}

// accumulateUsage 将 src 的各 token 计数累加到 dst
func accumulateUsage(dst *dto.Usage, src *dto.Usage) {
	if dst == nil || src == nil {
		return
	}
	dst.PromptTokens += src.PromptTokens
	dst.CompletionTokens += src.CompletionTokens
	dst.TotalTokens += src.TotalTokens
	dst.PromptCacheHitTokens += src.PromptCacheHitTokens
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.PromptTokensDetails.CachedTokens += src.PromptTokensDetails.CachedTokens
	dst.PromptTokensDetails.TextTokens += src.PromptTokensDetails.TextTokens
	dst.PromptTokensDetails.AudioTokens += src.PromptTokensDetails.AudioTokens
	dst.CompletionTokenDetails.ReasoningTokens += src.CompletionTokenDetails.ReasoningTokens
	dst.CompletionTokenDetails.TextTokens += src.CompletionTokenDetails.TextTokens
	dst.CompletionTokenDetails.AudioTokens += src.CompletionTokenDetails.AudioTokens
}
