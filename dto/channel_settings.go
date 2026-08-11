package dto

type ChannelSettings struct {
	ForceFormat            bool   `json:"force_format,omitempty"`
	ThinkingToContent      bool   `json:"thinking_to_content,omitempty"`
	Proxy                  string `json:"proxy"`
	PassThroughBodyEnabled bool   `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool   `json:"system_prompt_override,omitempty"`
	DisableProbe           bool   `json:"disable_probe,omitempty"` // 跳过批量探测/可用性测试
	Support4K              bool   `json:"support_4k,omitempty"`    // 是否支持4K图片生成
	// ResponsesToChatEnabled 命中后将 /v1/responses 请求降级转换为 /v1/chat/completions 发给上游，
	// 用于上游只支持 chat/completions、不支持 responses 端点的场景。
	ResponsesToChatEnabled bool `json:"responses_to_chat_enabled,omitempty"`
	// UnifyModelName 开启后，返回给客户端的响应体 model 字段统一改写为用户请求的模型名（OriginModelName），
	// 用于上游返回的模型名与请求名不一致/不稳定的场景。
	UnifyModelName          bool `json:"unify_model_name,omitempty"`
	KimiConvert             bool `json:"kimi_convert,omitempty"`
	// NFanoutEnabled 开启后，对上游不支持 n 参数的渠道（如 DeepSeek），
	// 网关会将客户端的 n>1 请求在内部拆成 N 个 n=1 的并发上游请求，
	// 再把结果合并为包含 N 条 choices 的响应返回，用量按 N 份累加。
	NFanoutEnabled bool `json:"n_fanout_enabled,omitempty"`
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

// AssetLibraryEndpointSettings describes a channel's upstream asset API.
// Paths may be absolute URLs or paths relative to BaseURL/the channel base URL.
type AssetLibraryEndpointSettings struct {
	Enabled         bool   `json:"enabled,omitempty"`
	BaseURL         string `json:"base_url,omitempty"`
	ListPath        string `json:"list_path,omitempty"`
	CreatePath      string `json:"create_path,omitempty"`
	DetailPath      string `json:"detail_path,omitempty"`
	AppendPath      string `json:"append_path,omitempty"`
	ImportURLPath   string `json:"import_url_path,omitempty"`
	ImportURLsPath  string `json:"import_urls_path,omitempty"`
	DeleteAssetPath string `json:"delete_asset_path,omitempty"`
}

type ChannelOtherSettings struct {
	AzureResponsesVersion                 string                        `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType                 `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool                         `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool                          `json:"claude_beta_query,omitempty"`         // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool                          `json:"allow_service_tier,omitempty"`        // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool                          `json:"allow_inference_geo,omitempty"`       // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool                          `json:"allow_speed,omitempty"`               // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool                          `json:"allow_safety_identifier,omitempty"`   // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool                          `json:"disable_store,omitempty"`             // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool                          `json:"allow_include_obfuscation,omitempty"` // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	AwsKeyType                            AwsKeyType                    `json:"aws_key_type,omitempty"`
	UpstreamModelUpdateCheckEnabled       bool                          `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool                          `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64                         `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string                      `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string                      `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string                      `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
	AssetLibrary                          *AssetLibraryEndpointSettings `json:"asset_library,omitempty"`
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
