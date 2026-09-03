package maas

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ---------------------------------------------------------------------------
// Model mapping cache
// ---------------------------------------------------------------------------

var (
	mappingCache   = map[string]string{}
	mappingCacheMu sync.RWMutex
)

func resolveEndpoint(baseURL, apiKey, modelName string) string {
	cacheKey := baseURL + "|" + modelName
	mappingCacheMu.RLock()
	if ep, ok := mappingCache[cacheKey]; ok {
		mappingCacheMu.RUnlock()
		return ep
	}
	mappingCacheMu.RUnlock()

	url := strings.TrimRight(baseURL, "/") + MappingQueryEndpoint
	body, _ := common.Marshal(map[string]string{"model": modelName})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return modelName
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := service.GetHttpClient().Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return modelName
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Endpoint string `json:"endpoint"`
	}
	if err := common.Unmarshal(respBody, &result); err != nil || result.Endpoint == "" {
		return modelName
	}

	mappingCacheMu.Lock()
	mappingCache[cacheKey] = result.Endpoint
	mappingCacheMu.Unlock()
	return result.Endpoint
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type requestPayload struct {
	Model         string        `json:"model"`
	Content       []ContentItem `json:"content,omitempty"`
	GenerateAudio *bool         `json:"generate_audio,omitempty"`
	Ratio         string        `json:"ratio,omitempty"`
	Duration      *int          `json:"duration,omitempty"`
	Watermark     *bool         `json:"watermark,omitempty"`
	Resolution    string        `json:"resolution,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"`
}

type responseTask struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Content struct {
		VideoURL     string `json:"video_url"`
		LastFrameURL string `json:"last_frame_url,omitempty"`
	} `json:"content"`
	// 上游若回显 usage（当前实测不返回，预留兼容）
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	// 上游若回显请求参数（本地估算 token 时优先采信）
	Resolution string `json:"resolution,omitempty"`
	Ratio      string `json:"ratio,omitempty"`
	Duration   int    `json:"duration,omitempty"`
	Error      struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// TaskAdaptor
// ---------------------------------------------------------------------------

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + CreateTaskEndpoint, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateTokenCount estimates the usage that will be available when the task
// completes. Dynamic pricing applies the reference-video distinction in the
// configured unit price, so it must use raw provider tokens instead of the
// legacy resource-package multiplier.
func (a *TaskAdaptor) EstimateTokenCount(c *gin.Context, info *relaycommon.RelayInfo) int {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return 0
	}
	common.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	body := a.convertToRequestPayload(&req, "")
	duration := derefInt(body.Duration)
	if billing_setting.GetBillingMode(info.OriginModelName) == billing_setting.BillingModeTieredExpr {
		return estimateSeedanceRawTokens(body.Resolution, body.Ratio, duration)
	}
	return estimateSeedanceTokens(body.Resolution, body.Ratio, duration, hasVideoInput(body.Content))
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = req.Model
	}
	resolvedEndpoint := resolveEndpoint(a.baseURL, a.apiKey, modelName)

	body := a.convertToRequestPayload(&req, resolvedEndpoint)
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}

	// store plain body for secure channel encryption in DoRequest
	c.Set("maas_plain_body", data)

	hasVideo := hasVideoInput(body.Content)
	if hasVideo {
		c.Set("maas_has_video", true)
	}

	// 持久化请求参数快照到 task.PrivateData.RequestSnapshot，供任务完成时本地估算 token。
	snap, snapErr := encodeRequestSnapshot(requestSnapshot{
		Resolution:    body.Resolution,
		Ratio:         body.Ratio,
		Duration:      derefInt(body.Duration),
		HasVideoInput: hasVideo,
	})
	if snapErr == nil {
		c.Set("task_request_snapshot", snap)
	}

	return bytes.NewReader(data), nil
}

// derefInt 安全解引用 *int；nil 返回 0。
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// DoRequest overrides the default to apply jeddak secure channel encryption.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	plainBody, _ := c.Get("maas_plain_body")
	plainBytes, ok := plainBody.([]byte)
	if !ok {
		// fallback: read from reader
		var err error
		plainBytes, err = io.ReadAll(requestBody)
		if err != nil {
			return nil, err
		}
	}

	pubKey, err := getServerPublicKey(a.baseURL, a.apiKey)
	if err != nil {
		return nil, fmt.Errorf("get server public key failed: %w", err)
	}

	encBody, aesKey, err := encryptBody(plainBytes, pubKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt body failed: %w", err)
	}

	reqURL := a.baseURL + CreateTaskEndpoint
	httpReq, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(encBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	httpReq.Header.Set("X-AICC-Encryption-Enable", "true")
	httpReq.Header.Set("X-AICC-Encryption-SDK", "aicc")
	httpReq.Header.Set("X-AICC-Encryption-Version", aiccSDKVersion)
	if _, hasVideo := c.Get("maas_has_video"); hasVideo {
		httpReq.Header.Set("Input-Has-Video", "true")
	}

	httpClient, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	// Decrypt response
	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	decrypted, err := decryptResponse(respBody, aesKey)
	if err != nil {
		// response may not be encrypted on error; return as-is
		decrypted = respBody
	}

	resp.Body = io.NopCloser(bytes.NewReader(decrypted))
	return resp, nil
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var mResp responsePayload
	if err := common.Unmarshal(responseBody, &mResp); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if mResp.ID == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("task_id is empty, body: %s", responseBody), "invalid_response", http.StatusInternalServerError)
	}

	// 火山原生入口：只返回 {"id": "<task_id>"}
	if strings.HasPrefix(c.Request.RequestURI, "/api/v3/contents/generations/tasks") {
		c.JSON(http.StatusOK, gin.H{"id": info.PublicTaskID})
		return mResp.ID, responseBody, nil
	}

	// OpenAI /v1/videos 兼容入口：返回 OpenAIVideo
	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)

	return mResp.ID, responseBody, nil
}

// FetchTask polls task status — also needs secure channel.
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	pubKey, err := getServerPublicKey(baseUrl, key)
	if err != nil {
		return nil, fmt.Errorf("get server public key failed: %w", err)
	}

	// SDK sends GET with body "{}"
	plainBytes := []byte("{}")
	encBody, aesKey, err := encryptBody(plainBytes, pubKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt body failed: %w", err)
	}

	uri := strings.TrimRight(baseUrl, "/") + fmt.Sprintf(QueryTaskEndpoint, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, bytes.NewReader(encBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-AICC-Encryption-Enable", "true")
	req.Header.Set("X-AICC-Encryption-SDK", "aicc")
	req.Header.Set("X-AICC-Encryption-Version", aiccSDKVersion)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	decrypted, err := decryptResponse(respBody, aesKey)
	if err != nil {
		decrypted = respBody
	}

	resp.Body = io.NopCloser(bytes.NewReader(decrypted))
	return resp, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var resTask responseTask
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := &relaycommon.TaskInfo{Code: 0}
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = taskcommon.ProgressQueued
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = taskcommon.ProgressInProgress
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = taskcommon.ProgressComplete
		taskResult.Url = resTask.Content.VideoURL
		// 仅在上游回显非零 usage 时采信；否则留给 AdjustBillingOnComplete 本地估算。
		if resTask.Usage.TotalTokens > 0 {
			taskResult.CompletionTokens = resTask.Usage.CompletionTokens
			taskResult.TotalTokens = resTask.Usage.TotalTokens
		}
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = taskcommon.ProgressComplete
		taskResult.Reason = resTask.Error.Message
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = taskcommon.ProgressInProgress
	}
	return taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal maas task data failed")
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = originTask.TaskID
	ov.TaskID = originTask.TaskID
	ov.Status = originTask.Status.ToVideoStatus()
	ov.SetProgressStr(originTask.Progress)
	ov.SetMetadata("url", dResp.Content.VideoURL)
	if dResp.Content.LastFrameURL != "" {
		ov.SetMetadata("last_frame_url", dResp.Content.LastFrameURL)
	}
	ov.CreatedAt = originTask.CreatedAt
	ov.CompletedAt = originTask.UpdatedAt
	ov.Model = originTask.Properties.OriginModelName

	if dResp.Status == "failed" {
		ov.Error = &dto.OpenAIVideoError{
			Message: dResp.Error.Message,
			Code:    dResp.Error.Code,
		}
	}
	return common.Marshal(ov)
}

// ConvertToVolcVideo 输出火山原生视频任务响应体（GET /api/v3/contents/generations/tasks/{id}）。
// 使用本地存的 originTask.Data（就是最近一次上游返回体）并覆盖 id 为本地 TaskID，
// 状态字段按火山官方枚举（queued/running/succeeded/failed）输出。
func (a *TaskAdaptor) ConvertToVolcVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if len(originTask.Data) > 0 {
		if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
			return nil, errors.Wrap(err, "unmarshal maas task data failed")
		}
	}
	dResp.ID = originTask.TaskID
	if dResp.Status == "" {
		dResp.Status = mapTaskStatusToVolc(originTask.Status)
	}
	return common.Marshal(dResp)
}

// mapTaskStatusToVolc 把本地任务状态映射到火山官方枚举。
func mapTaskStatusToVolc(s model.TaskStatus) string {
	switch s {
	case model.TaskStatusQueued, model.TaskStatusSubmitted, model.TaskStatusNotStart:
		return "queued"
	case model.TaskStatusInProgress:
		return "running"
	case model.TaskStatusSuccess:
		return "succeeded"
	case model.TaskStatusFailure:
		return "failed"
	default:
		return "queued"
	}
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }
func (a *TaskAdaptor) GetChannelName() string { return ChannelName }

// AdjustBillingOnComplete 在任务到达终态时被 settle 逻辑调用。
// 上游 MaaS Seedance 不会返回 usage 字段，这里按火山官方公式本地估算 token：
//
//	tokens = ceil(width × height × 24 × duration / 1024)
//	无视频输入场景再 × 1.6429
//
// 设置到 taskResult.TotalTokens 后返回 0，让通用 fallback 走"按 token 重算"路径。
// 已有 TotalTokens（上游回显或先前估算）则不覆盖。
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if taskResult == nil || taskResult.Status != model.TaskStatusSuccess {
		return 0
	}
	if taskResult.TotalTokens > 0 {
		return 0
	}

	// 1. 优先采信上游回显的请求参数（task.Data 在轮询时被覆盖为最新响应）
	var fromResp responseTask
	_ = common.Unmarshal(task.Data, &fromResp)

	// 2. 兜底读 BuildRequestBody 阶段持久化的快照
	snap, _ := decodeRequestSnapshot(task.PrivateData.RequestSnapshot)

	resolution := firstNonEmpty(fromResp.Resolution, snap.Resolution)
	ratio := firstNonEmpty(fromResp.Ratio, snap.Ratio)
	duration := fromResp.Duration
	if duration <= 0 {
		duration = snap.Duration
	}

	tokens := 0
	if bc := task.PrivateData.BillingContext; bc != nil && bc.TieredBillingSnapshot != nil {
		tokens = estimateSeedanceRawTokens(resolution, ratio, duration)
	} else {
		tokens = estimateSeedanceTokens(resolution, ratio, duration, snap.HasVideoInput)
	}
	if tokens <= 0 {
		return 0
	}
	taskResult.TotalTokens = tokens
	taskResult.CompletionTokens = tokens
	return 0
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, resolvedEndpoint string) *requestPayload {
	r := &requestPayload{
		Model:   resolvedEndpoint,
		Content: []ContentItem{},
	}

	_ = taskcommon.UnmarshalMetadata(req.Metadata, r)

	// 1) 顶层 content 若客户端直接以火山原生格式提交，直接反序列化到 r.Content
	if len(r.Content) == 0 && len(req.Content) > 0 {
		var items []ContentItem
		if err := common.Unmarshal(req.Content, &items); err == nil {
			r.Content = items
		}
	}

	// 2) 顶层火山字段兜底
	if r.Resolution == "" {
		r.Resolution = req.Resolution
	}
	if r.Ratio == "" {
		r.Ratio = req.Ratio
	}
	if r.Duration == nil && req.Duration > 0 {
		d := req.Duration
		r.Duration = &d
	}
	if r.GenerateAudio == nil && req.GenerateAudio != nil {
		r.GenerateAudio = req.GenerateAudio
	}
	if r.Watermark == nil && req.Watermark != nil {
		r.Watermark = req.Watermark
	}

	// 3) OpenAI /v1/videos 格式（size / seconds）映射到火山字段
	if r.Resolution == "" || r.Ratio == "" {
		if resolution, ratio, ok := parseOpenAISize(req.Size); ok {
			if r.Resolution == "" {
				r.Resolution = resolution
			}
			if r.Ratio == "" {
				r.Ratio = ratio
			}
		}
	}
	if r.Duration == nil && req.Seconds != "" {
		if d, err := strconv.Atoi(strings.TrimSpace(req.Seconds)); err == nil && d > 0 {
			r.Duration = &d
		}
	}

	// 4) 兜底：如果 content 里没有文本项，把 prompt 塞进去
	hasText := false
	for _, item := range r.Content {
		if item.Type == "text" {
			hasText = true
			break
		}
	}
	if !hasText && req.Prompt != "" {
		r.Content = append(r.Content, ContentItem{Type: "text", Text: req.Prompt})
	}

	return r
}

// parseOpenAISize 把 OpenAI 视频 API 的 size 字段（如 "1280x720"）映射到
// 火山的 resolution + ratio。识别失败时 ok=false。
//
// resolution 由较短边决定（火山文档：480p/720p/1080p/4k 对应短边像素）。
// ratio 由 (宽,高) 归约后的最简比得到，缺省 16:9。
func parseOpenAISize(size string) (resolution, ratio string, ok bool) {
	s := strings.TrimSpace(strings.ToLower(size))
	if s == "" {
		return "", "", false
	}
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return "", "", false
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return "", "", false
	}

	short := w
	if h < short {
		short = h
	}
	switch {
	case short <= 480:
		resolution = "480p"
	case short <= 720:
		resolution = "720p"
	case short <= 1080:
		resolution = "1080p"
	default:
		resolution = "4k"
	}

	g := gcd(w, h)
	rw, rh := w/g, h/g
	// 归一常见比例
	switch fmt.Sprintf("%d:%d", rw, rh) {
	case "16:9", "9:16", "4:3", "3:4", "1:1", "21:9", "9:21":
		ratio = fmt.Sprintf("%d:%d", rw, rh)
	default:
		ratio = "16:9"
	}
	return resolution, ratio, true
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func hasVideoInput(content []ContentItem) bool {
	for _, item := range content {
		if item.Type == "video_url" {
			return true
		}
	}
	return false
}
