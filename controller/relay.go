package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			logger.LogError(c, fmt.Sprintf("relay error: %s", newAPIError.Error()))
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(),
				})
			default:
				c.JSON(newAPIError.StatusCode, gin.H{
					"error": newAPIError.ToOpenAIError(),
				})
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			newAPIError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = service.NormalizeViolationFeeError(newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			newAPIError = channelErr
			break
		}

		addUsedChannel(c, channel.Id, false)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil
			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError)

		if !shouldRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	// ===== 兜底阶段 =====
	if newAPIError != nil {
		originalError := newAPIError
		usedChannels := c.GetStringSlice("use_channel")

		fallbackChannel, fallbackErr := service.GetFallbackChannel(relayInfo.OriginModelName, usedChannels)
		if fallbackErr == nil && fallbackChannel != nil {
			logger.LogInfo(c, fmt.Sprintf("触发兜底策略：原渠道全部失败（已重试%d次），尝试兜底渠道 #%d (%s)",
				common.RetryTimes, fallbackChannel.Id, fallbackChannel.Name))

			// 退还原渠道预扣费
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
				relayInfo.Billing = nil
			}

			// 设置兜底渠道上下文
			setupErr := middleware.SetupContextForSelectedChannel(c, fallbackChannel, relayInfo.OriginModelName)
			if setupErr == nil {
				// 重新计算兜底渠道价格
				fallbackPriceData, priceErr := helper.ModelPriceHelper(c, relayInfo, relayInfo.GetEstimatePromptTokens(), meta)
				if priceErr == nil {
					// 按兜底渠道价格预扣费
					if !fallbackPriceData.FreeModel {
						preConsumeErr := service.PreConsumeBilling(c, fallbackPriceData.QuotaToPreConsume, relayInfo)
						if preConsumeErr == nil {
							// 重置请求体
							bodyStorage, bodyErr := common.GetBodyStorage(c)
							if bodyErr == nil {
								c.Request.Body = io.NopCloser(bodyStorage)

								// 记录兜底渠道
								addUsedChannel(c, fallbackChannel.Id, true)

								// 执行兜底请求
								switch relayFormat {
								case types.RelayFormatOpenAIRealtime:
									newAPIError = relay.WssHelper(c, relayInfo)
								case types.RelayFormatClaude:
									newAPIError = relay.ClaudeHelper(c, relayInfo)
								case types.RelayFormatGemini:
									newAPIError = geminiRelayHandler(c, relayInfo)
								default:
									newAPIError = relayHandler(c, relayInfo)
								}

								if newAPIError == nil {
									// 兜底成功
									logger.LogInfo(c, fmt.Sprintf("兜底请求成功：渠道 #%d，原始错误：%s",
										fallbackChannel.Id, originalError.Error()))
									relayInfo.LastError = nil
									return
								} else {
									// 兜底失败
									logger.LogError(c, fmt.Sprintf("兜底请求失败：渠道 #%d，兜底错误：%s，原始错误：%s",
										fallbackChannel.Id, newAPIError.Error(), originalError.Error()))
									processChannelError(c, *types.NewChannelError(fallbackChannel.Id, fallbackChannel.Type, fallbackChannel.Name,
										fallbackChannel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey),
										fallbackChannel.GetAutoBan()), newAPIError)
								}
							} else {
								logger.LogError(c, fmt.Sprintf("兜底渠道获取请求体失败：%s", bodyErr.Error()))
							}
						} else {
							logger.LogError(c, fmt.Sprintf("兜底渠道预扣费失败：%s", preConsumeErr.Error()))
						}
					} else {
						// 兜底渠道免费，直接执行请求
						bodyStorage, bodyErr := common.GetBodyStorage(c)
						if bodyErr == nil {
							c.Request.Body = io.NopCloser(bodyStorage)
							addUsedChannel(c, fallbackChannel.Id, true)

							switch relayFormat {
							case types.RelayFormatOpenAIRealtime:
								newAPIError = relay.WssHelper(c, relayInfo)
							case types.RelayFormatClaude:
								newAPIError = relay.ClaudeHelper(c, relayInfo)
							case types.RelayFormatGemini:
								newAPIError = geminiRelayHandler(c, relayInfo)
							default:
								newAPIError = relayHandler(c, relayInfo)
							}

							if newAPIError == nil {
								logger.LogInfo(c, fmt.Sprintf("兜底请求成功（免费渠道）：渠道 #%d", fallbackChannel.Id))
								relayInfo.LastError = nil
								return
							} else {
								logger.LogError(c, fmt.Sprintf("兜底请求失败（免费渠道）：渠道 #%d，错误：%s",
									fallbackChannel.Id, newAPIError.Error()))
								processChannelError(c, *types.NewChannelError(fallbackChannel.Id, fallbackChannel.Type, fallbackChannel.Name,
									fallbackChannel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey),
									fallbackChannel.GetAutoBan()), newAPIError)
							}
						}
					}
				} else {
					logger.LogError(c, fmt.Sprintf("兜底渠道价格计算失败：%s", priceErr.Error()))
				}
			} else {
				logger.LogError(c, fmt.Sprintf("兜底渠道上下文设置失败：%s", setupErr.Error()))
			}
		}
		// 恢复原始错误
		newAPIError = originalError
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
	if newAPIError != nil {
		gopool.Go(func() {
			perfmetrics.RecordRelaySample(relayInfo, false, 0)
		})
	}
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int, isFallback bool) {
	useChannel := c.GetStringSlice("use_channel")
	if isFallback {
		useChannel = append(useChannel, fmt.Sprintf("fallback:%d", channelId))
	} else {
		useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	}
	c.Set("use_channel", useChannel)
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if info.ChannelMeta == nil {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		return &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func shouldRetry(c *gin.Context, openaiErr *types.NewAPIError, retryTimes int) bool {
	if openaiErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, err.Error()))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously
	if service.ShouldDisableChannel(err) && channelError.AutoBan {
		gopool.Go(func() {
			service.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelId := c.GetInt("channel_id")
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		service.AppendChannelAffinityAdminInfo(c, adminInfo)
		other["admin_info"] = adminInfo
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, other)
	}

}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}

		addUsedChannel(c, channel.Id, false)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}

		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
		}

		if !shouldRetryTaskRelay(c, channel.Id, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	// ===== Task API 兜底阶段 =====
	if taskErr != nil {
		originalTaskErr := taskErr
		usedChannels := c.GetStringSlice("use_channel")

		fallbackChannel, fallbackErr := service.GetFallbackChannel(relayInfo.OriginModelName, usedChannels)
		if fallbackErr == nil && fallbackChannel != nil {
			logger.LogInfo(c, fmt.Sprintf("触发兜底策略(Task)：原渠道全部失败（已重试%d次），尝试兜底渠道 #%d (%s)",
				common.RetryTimes, fallbackChannel.Id, fallbackChannel.Name))

			// 退还原渠道预扣费
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
				relayInfo.Billing = nil
			}

			// 设置兜底渠道上下文
			setupErr := middleware.SetupContextForSelectedChannel(c, fallbackChannel, relayInfo.OriginModelName)
			if setupErr == nil {
				// 重新计算兜底渠道价格
				fallbackPriceData, priceErr := helper.ModelPriceHelper(c, relayInfo, relayInfo.GetEstimatePromptTokens(), nil)
				if priceErr == nil {
					// 按兜底渠道价格预扣费
					if !fallbackPriceData.FreeModel {
						preConsumeErr := service.PreConsumeBilling(c, fallbackPriceData.QuotaToPreConsume, relayInfo)
						if preConsumeErr == nil {
							// 重置请求体
							bodyStorage, bodyErr := common.GetBodyStorage(c)
							if bodyErr == nil {
								c.Request.Body = io.NopCloser(bodyStorage)

								// 记录兜底渠道
								addUsedChannel(c, fallbackChannel.Id, true)

								// 执行兜底请求
								result, taskErr = relay.RelayTaskSubmit(c, relayInfo)

								if taskErr == nil {
									// 兜底成功
									logger.LogInfo(c, fmt.Sprintf("兜底请求成功(Task)：渠道 #%d", fallbackChannel.Id))
								} else {
									// 兜底失败
									logger.LogError(c, fmt.Sprintf("兜底请求失败(Task)：渠道 #%d，兜底错误：%s，原始错误：%s",
										fallbackChannel.Id, taskErr.Message, originalTaskErr.Message))
									if !taskErr.LocalError {
										processChannelError(c,
											*types.NewChannelError(fallbackChannel.Id, fallbackChannel.Type, fallbackChannel.Name,
												fallbackChannel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey),
												fallbackChannel.GetAutoBan()),
											types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
									}
								}
							} else {
								logger.LogError(c, fmt.Sprintf("兜底渠道获取请求体失败(Task)：%s", bodyErr.Error()))
							}
						} else {
							logger.LogError(c, fmt.Sprintf("兜底渠道预扣费失败(Task)：%s", preConsumeErr.Error()))
						}
					} else {
						// 兜底渠道免费，直接执行请求
						bodyStorage, bodyErr := common.GetBodyStorage(c)
						if bodyErr == nil {
							c.Request.Body = io.NopCloser(bodyStorage)
							addUsedChannel(c, fallbackChannel.Id, true)

							result, taskErr = relay.RelayTaskSubmit(c, relayInfo)

							if taskErr == nil {
								logger.LogInfo(c, fmt.Sprintf("兜底请求成功(Task,免费渠道)：渠道 #%d", fallbackChannel.Id))
							} else {
								logger.LogError(c, fmt.Sprintf("兜底请求失败(Task,免费渠道)：渠道 #%d，错误：%s",
									fallbackChannel.Id, taskErr.Message))
								if !taskErr.LocalError {
									processChannelError(c,
										*types.NewChannelError(fallbackChannel.Id, fallbackChannel.Type, fallbackChannel.Name,
											fallbackChannel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey),
											fallbackChannel.GetAutoBan()),
										types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
								}
							}
						}
					}
				} else {
					logger.LogError(c, fmt.Sprintf("兜底渠道价格计算失败(Task)：%s", priceErr.Error()))
				}
			} else {
				logger.LogError(c, fmt.Sprintf("兜底渠道上下文设置失败(Task)：%s", setupErr.Error()))
			}
		}
		// 恢复原始错误（如果兜底失败）
		if taskErr != nil {
			taskErr = originalTaskErr
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		initialQuota := result.Quota
		if result.DeferredBilling {
			initialQuota = 0
		} else if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}
		// 延迟计费任务在完成时才写消费日志和累加请求计数，提交阶段不写。
		if !result.DeferredBilling {
			service.LogTaskConsumption(c, relayInfo, initialQuota)
		}

		task := model.InitTask(result.Platform, relayInfo)
		// 记录用户提示词到 Properties.Input —— 前端历史列表用它显示原始提问。
		if taskReq, taskReqErr := relaycommon.GetTaskRequest(c); taskReqErr == nil {
			task.Properties.Input = taskReq.Prompt
		}
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		// 通用扩展点：adaptor 可在 BuildRequestBody 阶段通过 c.Set("task_request_snapshot", []byte{...})
		// 将请求参数快照写入 task.PrivateData.RequestSnapshot，供轮询/结算阶段使用。
		// 其他 channel 不设置时此字段为 nil，无副作用。
		if snap, exists := c.Get("task_request_snapshot"); exists {
			if b, ok := snap.([]byte); ok && len(b) > 0 {
				task.PrivateData.RequestSnapshot = b
			}
		}
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:            relayInfo.PriceData.ModelPrice,
			GroupRatio:            relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:            relayInfo.PriceData.ModelRatio,
			OtherRatios:           relayInfo.PriceData.OtherRatios,
			OriginModelName:       relayInfo.OriginModelName,
			PerCallBilling:        relayInfo.TieredBillingSnapshot == nil && (common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice),
			DeferredBilling:       result.DeferredBilling,
			EstimatedQuota:        result.Quota,
			BillingPreference:     common.NormalizeBillingPreference(relayInfo.UserSetting.BillingPreference),
			TieredBillingSnapshot: relayInfo.TieredBillingSnapshot,
		}
		if relayInfo.BillingRequestInput != nil && len(relayInfo.BillingRequestInput.Body) > 0 {
			task.PrivateData.BillingContext.BillingRequestBody = append(
				task.PrivateData.BillingContext.BillingRequestBody,
				relayInfo.BillingRequestInput.Body...,
			)
		}
		task.Quota = initialQuota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}
