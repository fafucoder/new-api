package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

// contentFilterKeywords 用于判定上游错误是否属于内容过滤 / 安全拦截类。
// 匹配时统一转小写，命中任意一个即视为内容过滤错误。
var contentFilterKeywords = []string{
	"content_filter",
	"content-filter",
	"contentfilter",
	"content_policy",
	"content policy",
	"content management",
	"data_inspection_failed",
	"risk",
	"sensitive",
	"safety",
	"违规",
	"敏感",
	"安全",
}

// IsContentFilterError 判断一个上游错误是否为内容过滤 / 安全拦截类错误。
// 用于「关闭滤网拦截」开关：仅当命中内容过滤特征时才吞掉错误伪装成正常响应，
// 其它类型的错误（参数错误、鉴权失败等）不受影响，照常返回给客户端。
func IsContentFilterError(newApiErr *types.NewAPIError) bool {
	if newApiErr == nil {
		return false
	}
	oaiErr := newApiErr.ToOpenAIError()
	candidates := []string{
		strings.ToLower(oaiErr.Type),
		strings.ToLower(oaiErr.Message),
		strings.ToLower(fmt.Sprintf("%v", oaiErr.Code)),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		for _, keyword := range contentFilterKeywords {
			if strings.Contains(candidate, keyword) {
				return true
			}
		}
	}
	return false
}
