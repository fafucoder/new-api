package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/image_size_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// imageSizeValidationBody 仅用于从请求体中提取校验所需的最小字段。
// 兼容 JSON 与 multipart/form-data（由 common.UnmarshalBodyReusable 内部处理）。
type imageSizeValidationBody struct {
	Model string `json:"model" form:"model"`
	Size  string `json:"size" form:"size"`
}

// ImageSizeValidation 生图接口专用中间件：对配置指定的模型（如 gpt-image-2）
// 的 size 参数做硬性尺寸校验。校验失败时写入错误日志并直接返回 400，不转发上游。
// 仅挂载在 image 相关路由上，不影响其他接口。
func ImageSizeValidation() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 总开关未开启时快速放行，避免无谓解析请求体
		if !image_size_setting.GetValidationSetting().Enabled {
			c.Next()
			return
		}

		var body imageSizeValidationBody
		if err := common.UnmarshalBodyReusable(c, &body); err != nil {
			// 解析失败不在此拦截，交由后续 relay 流程处理，保持既有行为
			c.Next()
			return
		}

		if !image_size_setting.ShouldValidateModel(body.Model) {
			c.Next()
			return
		}

		if err := image_size_setting.ValidateSize(body.Size); err != nil {
			recordImageSizeValidationError(c, body.Model, err)
			abortWithOpenAiMessage(c, http.StatusBadRequest, err.Error(), types.ErrorCodeInvalidRequest)
			return
		}

		c.Next()
	}
}

// recordImageSizeValidationError 将尺寸校验失败写入错误日志表（LogTypeError），
// 使后台「使用日志」能看到 status_code=400 及失败原因。
func recordImageSizeValidationError(c *gin.Context, modelName string, err error) {
	userId := c.GetInt("id")
	channelId := c.GetInt("channel_id")
	tokenId := c.GetInt("token_id")
	tokenName := c.GetString("token_name")
	group := c.GetString("group")
	if originModel := c.GetString("original_model"); originModel != "" {
		modelName = originModel
	}

	other := make(map[string]any)
	if c.Request != nil && c.Request.URL != nil {
		other["request_path"] = c.Request.URL.Path
	}
	other["error_type"] = string(types.ErrorTypeNewAPIError)
	other["error_code"] = string(types.ErrorCodeInvalidRequest)
	other["status_code"] = http.StatusBadRequest
	other["channel_id"] = channelId
	other["channel_name"] = c.GetString("channel_name")
	other["channel_type"] = c.GetInt("channel_type")

	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	useTimeSeconds := int(time.Since(startTime).Seconds())

	content := fmt.Sprintf("status_code=%d, %s", http.StatusBadRequest, err.Error())
	model.RecordErrorLog(c, userId, channelId, modelName, tokenName, content, tokenId, useTimeSeconds, false, group, other)
}
