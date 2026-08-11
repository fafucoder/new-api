package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// taskTokenWritebackPlatforms 平台白名单：任务完成后把本地估算的 token 数
// 回写到原 log 行的 completion_tokens 列，让统计页面正确显示消耗。
// 不在此名单内的平台保持原行为（log 行 tokens=0，但 quota 计费仍走 RecalculateTaskQuotaByTokens）。
// 需要新增平台时在此追加。
var taskTokenWritebackPlatforms = map[constant.TaskPlatform]bool{
	// MaaS Seedance 2.0 文生视频：上游不返回 usage，由 adaptor 本地按火山公式估算 token。
	constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeMaasSeedance)): true,
}

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo, billedQuota int) {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	// 支持任务仅按次计费
	if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		logContent = fmt.Sprintf("%s，按次计费", logContent)
	} else {
		if len(info.PriceData.OtherRatios) > 0 {
			var contents []string
			for key, ra := range info.PriceData.OtherRatios {
				if 1.0 != ra {
					contents = append(contents, fmt.Sprintf("%s: %.2f", key, ra))
				}
			}
			if len(contents) > 0 {
				logContent = fmt.Sprintf("%s, 计算参数：%s", logContent, strings.Join(contents, ", "))
			}
		}
	}
	other := make(map[string]interface{})
	other["is_task"] = true
	other["request_path"] = c.Request.URL.Path
	// 记录 task_id 用于任务完成时回写 token 列（RecalculateTaskQuotaByTokens 会用到）
	if info.TaskRelayInfo != nil && info.TaskRelayInfo.PublicTaskID != "" {
		other["task_id"] = info.TaskRelayInfo.PublicTaskID
	}
	other["model_price"] = info.PriceData.ModelPrice
	if billedQuota != info.PriceData.Quota {
		other["deferred_billing"] = true
		other["estimated_quota"] = info.PriceData.Quota
	}
	if info.PriceData.ModelRatio > 0 {
		other["model_ratio"] = info.PriceData.ModelRatio
	}
	other["group_ratio"] = info.PriceData.GroupRatioInfo.GroupRatio
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}
	adminInfo := map[string]interface{}{
		"use_channel": c.GetStringSlice("use_channel"),
	}
	if common.GetContextKeyBool(c, constant.ContextKeyLocalCountTokens) {
		adminInfo["local_count_tokens"] = true
	}
	other["admin_info"] = adminInfo
	appendRequestConversionChain(info, other)
	appendFinalRequestFormat(info, other)
	appendBillingInfo(info, other)
	InjectTieredBillingInfo(other, info, nil)
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId: info.ChannelId,
		ModelName: info.OriginModelName,
		TokenName: tokenName,
		Quota:     billedQuota,
		Content:   logContent,
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	})
	model.UpdateUserUsedQuotaAndRequestCount(info.UserId, billedQuota)
	model.UpdateChannelUsedQuota(info.ChannelId, billedQuota)
}

// ---------------------------------------------------------------------------
// 异步任务计费辅助函数
// ---------------------------------------------------------------------------

// resolveTokenKey 通过 TokenId 运行时获取令牌 Key（用于 Redis 缓存操作）。
// 如果令牌已被删除或查询失败，返回空字符串。
func resolveTokenKey(ctx context.Context, tokenId int, taskID string) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("获取令牌 key 失败 (tokenId=%d, task=%s): %s", tokenId, taskID, err.Error()))
		return ""
	}
	return token.Key
}

// taskIsSubscription 判断任务是否通过订阅计费。
func taskIsSubscription(task *model.Task) bool {
	return task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
}

func chargeDeferredTaskFunding(task *model.Task, quota int) error {
	preference := ""
	if task.PrivateData.BillingContext != nil {
		preference = task.PrivateData.BillingContext.BillingPreference
	}
	preference = common.NormalizeBillingPreference(preference)

	chargeWallet := func() error {
		if err := model.DecreaseUserQuota(task.UserId, quota, false); err != nil {
			return err
		}
		task.PrivateData.BillingSource = BillingSourceWallet
		task.PrivateData.SubscriptionId = 0
		return nil
	}
	chargeSubscription := func() error {
		requestID := "task_settle_" + strings.TrimPrefix(task.TaskID, "task_")
		result, err := model.PreConsumeUserSubscription(
			requestID,
			task.UserId,
			taskModelName(task),
			0,
			int64(quota),
		)
		if err != nil {
			return err
		}
		task.PrivateData.BillingSource = BillingSourceSubscription
		task.PrivateData.SubscriptionId = result.UserSubscriptionId
		return nil
	}

	switch preference {
	case "subscription_only":
		return chargeSubscription()
	case "wallet_only":
		return chargeWallet()
	case "wallet_first":
		walletQuota, err := model.GetUserQuota(task.UserId, false)
		if err != nil {
			return err
		}
		if walletQuota >= quota {
			return chargeWallet()
		}
		if err := chargeSubscription(); err == nil {
			return nil
		}
		// The task already succeeded. Keep the charge instead of leaving it unpaid
		// when neither preferred source currently has enough quota.
		return chargeWallet()
	case "subscription_first":
		if err := chargeSubscription(); err == nil {
			return nil
		}
		return chargeWallet()
	default:
		return chargeWallet()
	}
}

// taskAdjustFunding 调整任务的资金来源（钱包或订阅），delta > 0 表示扣费，delta < 0 表示退还。
func taskAdjustFunding(task *model.Task, delta int) error {
	if delta > 0 && task.PrivateData.BillingSource == "" {
		if bc := task.PrivateData.BillingContext; bc != nil && bc.DeferredBilling {
			return chargeDeferredTaskFunding(task, delta)
		}
	}
	if taskIsSubscription(task) {
		return model.PostConsumeUserSubscriptionDelta(task.PrivateData.SubscriptionId, int64(delta))
	}
	if delta > 0 {
		return model.DecreaseUserQuota(task.UserId, delta, false)
	}
	return model.IncreaseUserQuota(task.UserId, -delta, false)
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) {
	if task.PrivateData.TokenId <= 0 || delta == 0 {
		return
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return
	}
	var err error
	if delta > 0 {
		err = model.DecreaseTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
	} else {
		err = model.IncreaseTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
	}
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
}

// taskBillingOther 从 task 的 BillingContext 构建日志 Other 字段。
func taskBillingOther(task *model.Task) map[string]interface{} {
	other := make(map[string]interface{})
	if bc := task.PrivateData.BillingContext; bc != nil {
		other["model_price"] = bc.ModelPrice
		if bc.ModelRatio > 0 {
			other["model_ratio"] = bc.ModelRatio
		}
		other["group_ratio"] = bc.GroupRatio
		if len(bc.OtherRatios) > 0 {
			for k, v := range bc.OtherRatios {
				other[k] = v
			}
		}
		if snap := bc.TieredBillingSnapshot; snap != nil {
			other["billing_mode"] = "tiered_expr"
			other["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
			if snap.EstimatedTier != "" {
				other["matched_tier"] = snap.EstimatedTier
			}
		}
		if bc.DeferredBilling {
			other["deferred_billing"] = true
			other["estimated_quota"] = bc.EstimatedQuota
			other["billing_preference"] = bc.BillingPreference
		}
	}
	if task.PrivateData.BillingSource != "" {
		other["billing_source"] = task.PrivateData.BillingSource
	}
	if task.PrivateData.SubscriptionId > 0 {
		other["subscription_id"] = task.PrivateData.SubscriptionId
	}
	props := task.Properties
	if props.UpstreamModelName != "" && props.UpstreamModelName != props.OriginModelName {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = props.UpstreamModelName
	}
	return other
}

// taskModelName 从 BillingContext 或 Properties 中获取模型名称。
func taskModelName(task *model.Task) string {
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OriginModelName != "" {
		return bc.OriginModelName
	}
	return task.Properties.OriginModelName
}

// RefundTaskQuota 统一的任务失败退款逻辑。
// 当异步任务失败时，将预扣的 quota 退还给用户（支持钱包和订阅），并退还令牌额度。
func RefundTaskQuota(ctx context.Context, task *model.Task, reason string) {
	quota := task.Quota
	if quota == 0 {
		return
	}

	// 1. 退还资金来源（钱包或订阅）
	if err := taskAdjustFunding(task, -quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还资金来源失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 2. 退还令牌额度
	taskAdjustTokenQuota(ctx, task, -quota)

	// 3. 记录日志
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecalculateTaskQuota 通用的异步差额结算。
// actualQuota 是任务完成后的实际应扣额度，与预扣额度 (task.Quota) 做差额结算。
// reason 用于日志记录（例如 "token重算" 或 "adaptor调整"）。
func RecalculateTaskQuota(ctx context.Context, task *model.Task, actualQuota int, reason string) {
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota

	if quotaDelta == 0 {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）",
			task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))

	// 调整资金来源
	if err := taskAdjustFunding(task, quotaDelta); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 调整令牌额度
	taskAdjustTokenQuota(ctx, task, quotaDelta)

	task.Quota = actualQuota
	if err := task.UpdateBillingState(); err != nil {
		logger.LogError(ctx, fmt.Sprintf("持久化任务计费状态失败 task %s: %s", task.TaskID, err.Error()))
	}

	var logType int
	var logQuota int
	model.UpdateUserUsedQuota(task.UserId, quotaDelta)
	model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)
	if username, err := model.GetUsernameById(task.UserId, false); err == nil {
		model.IncreaseQuotaDataQuota(task.UserId, username, taskModelName(task), task.SubmitTime, quotaDelta)
	}
	if quotaDelta > 0 {
		logType = model.LogTypeConsume
		logQuota = quotaDelta
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = actualQuota
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   logType,
		Content:   reason,
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     logQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecalculateTaskQuotaByTokens 根据实际 token 消耗重新计费（异步差额结算）。
// 当任务成功且返回了 totalTokens 时，根据模型倍率和分组倍率重新计算实际扣费额度，
// 与预扣费的差额进行补扣或退还。支持钱包和订阅计费来源。
func RecalculateTaskQuotaByTokens(ctx context.Context, task *model.Task, totalTokens int) {
	if totalTokens <= 0 {
		return
	}

	modelName := taskModelName(task)
	if bc := task.PrivateData.BillingContext; bc != nil && bc.TieredBillingSnapshot != nil {
		requestInput := billingexpr.RequestInput{Body: bc.BillingRequestBody}
		result, err := billingexpr.ComputeTieredQuotaWithRequest(
			bc.TieredBillingSnapshot,
			billingexpr.TokenParams{C: float64(totalTokens)},
			requestInput,
		)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("任务 %s 动态计费结算失败: %s", task.TaskID, err.Error()))
			return
		}

		reason := fmt.Sprintf("动态计费结算：tokens=%d, tier=%s, groupRatio=%.4f", totalTokens, result.MatchedTier, bc.TieredBillingSnapshot.GroupRatio)
		RecalculateTaskQuota(ctx, task, result.ActualQuotaAfterGroup, reason)

		other := map[string]interface{}{
			"billing_mode": "tiered_expr",
			"expr_b64":     base64.StdEncoding.EncodeToString([]byte(bc.TieredBillingSnapshot.ExprString)),
			"matched_tier": result.MatchedTier,
			"group_ratio":  bc.TieredBillingSnapshot.GroupRatio,
		}
		videoInfo, isVideoRequest := billingexpr.ExtractVideoRequestInfo(requestInput)
		if tierInfo, ok := billingexpr.ParseVideoTierLabel(result.MatchedTier); ok {
			isVideoRequest = true
			if videoInfo.Resolution == "" || tierInfo.Default {
				videoInfo.Resolution = tierInfo.Resolution
			}
			videoInfo.HasReferenceVideo = tierInfo.HasReferenceVideo
		}
		if isVideoRequest {
			unitPriceUSD := 0.0
			if totalTokens > 0 {
				unitPriceUSD = result.ActualCost / float64(totalTokens)
			}
			amountBeforeGroupUSD := result.ActualCost / 1_000_000
			deductedAmountUSD := 0.0
			if bc.TieredBillingSnapshot.QuotaPerUnit > 0 {
				deductedAmountUSD = float64(result.ActualQuotaAfterGroup) / bc.TieredBillingSnapshot.QuotaPerUnit
			}
			videoBilling := map[string]interface{}{
				"resolution":              videoInfo.Resolution,
				"reference_video":         videoInfo.HasReferenceVideo,
				"tokens":                  totalTokens,
				"unit_price_usd":          unitPriceUSD,
				"amount_before_group_usd": amountBeforeGroupUSD,
				"group_ratio":             bc.TieredBillingSnapshot.GroupRatio,
				"final_amount_usd":        amountBeforeGroupUSD * bc.TieredBillingSnapshot.GroupRatio,
				"deducted_quota":          result.ActualQuotaAfterGroup,
				"deducted_amount_usd":     deductedAmountUSD,
			}
			if videoURL := task.GetResultURL(); videoURL != "" {
				videoBilling["video_url"] = videoURL
			}
			other["video_billing"] = videoBilling
		}

		content := "Task completion settlement"
		if strings.Contains(strings.ToLower(modelName), "seedance") {
			content = "Seedance completion settlement"
		}
		writeBackTaskUsage(ctx, task, totalTokens, content, other, true)
		return
	}

	// 获取模型价格和倍率
	modelRatio, hasRatioSetting, _ := ratio_setting.GetModelRatio(modelName)
	// 只有配置了倍率(非固定价格)时才按 token 重新计费
	if !hasRatioSetting || modelRatio <= 0 {
		return
	}

	// 获取用户和组的倍率信息
	group := task.Group
	if group == "" {
		user, err := model.GetUserById(task.UserId, false)
		if err == nil {
			group = user.Group
		}
	}
	if group == "" {
		return
	}

	groupRatio := ratio_setting.GetGroupRatio(group)
	userGroupRatio, hasUserGroupRatio := ratio_setting.GetGroupGroupRatio(group, group)

	var finalGroupRatio float64
	if hasUserGroupRatio {
		finalGroupRatio = userGroupRatio
	} else {
		finalGroupRatio = groupRatio
	}

	// 计算 OtherRatios 乘积（视频折扣、时长等）
	otherMultiplier := 1.0
	if bc := task.PrivateData.BillingContext; bc != nil {
		for _, r := range bc.OtherRatios {
			if r != 1.0 && r > 0 {
				otherMultiplier *= r
			}
		}
	}

	// 计算实际应扣费额度: totalTokens * modelRatio * groupRatio * otherMultiplier
	actualQuota := int(float64(totalTokens) * modelRatio * finalGroupRatio * otherMultiplier)

	reason := fmt.Sprintf("token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f, otherMultiplier=%.4f", totalTokens, modelRatio, finalGroupRatio, otherMultiplier)
	RecalculateTaskQuota(ctx, task, actualQuota, reason)

	// 白名单平台：把本地估算的 token 数回写到原 consume 日志行的 completion_tokens 列，
	// 让统计页面能正确显示消耗 token 数（quota 计费已由上一步处理）。
	writeBackTaskUsage(ctx, task, totalTokens, "", nil, false)
}

func writeBackTaskUsage(ctx context.Context, task *model.Task, totalTokens int, content string, other map[string]interface{}, force bool) {
	if force || taskTokenWritebackPlatforms[task.Platform] {
		rows, err := model.UpdateTaskLogSettlement(task.UserId, task.ChannelId, task.TaskID, totalTokens, content, other)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("回写任务日志 completion_tokens 失败 task=%s: %s", task.TaskID, err.Error()))
		} else if rows == 0 {
			logger.LogWarn(ctx, fmt.Sprintf("回写任务日志 completion_tokens 未命中 task=%s", task.TaskID))
		}
		// 同步补一份到仪表盘聚合表 quota_data（按 SubmitTime 的小时桶）。
		// 提交时 LogQuotaData 已经写过 tokenUsed=0，这里按差额补 totalTokens。
		if username, uerr := model.GetUsernameById(task.UserId, false); uerr == nil {
			model.IncreaseQuotaDataTokenUsed(task.UserId, username, taskModelName(task), task.SubmitTime, totalTokens)
		}
	}
}
