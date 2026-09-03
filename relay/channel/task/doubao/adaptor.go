package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

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
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content,omitempty"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
	Ratio       string         `json:"ratio,omitempty"`
	Duration    *dto.IntValue  `json:"duration,omitempty"`
	Frames      *dto.IntValue  `json:"frames,omitempty"`
	Seed        *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark   *dto.BoolValue `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL     string `json:"video_url"`
		LastFrameURL string `json:"last_frame_url,omitempty"`
	} `json:"content"`
	Seed            int    `json:"seed"`
	Resolution      string `json:"resolution"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	FramesPerSecond int    `json:"framespersecond"`
	ServiceTier     string `json:"service_tier"`
	Tools           []struct {
		Type string `json:"type"`
	} `json:"tools"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		ToolUsage        struct {
			WebSearch int `json:"web_search"`
		} `json:"tool_usage"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction parses body, validates fields and sets default action.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// Accept only POST /v1/video/generations as "generate" action.
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateBilling 检测请求 metadata 中是否包含视频输入，返回视频折扣 OtherRatio。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	if hasVideoInMetadata(req.Metadata) {
		if ratio, ok := GetVideoInputRatio(info.OriginModelName); ok {
			return map[string]float64{"video_input": ratio}
		}
	}
	return nil
}

// hasVideoInMetadata 直接检查 metadata 的 content 数组是否包含 video_url 条目，
// 避免构建完整的上游 requestPayload。
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return false
	}
	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range contentSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemMap["type"] == "video_url" {
			return true
		}
		if _, has := itemMap["video_url"]; has {
			return true
		}
	}
	return false
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 火山原生入口 /api/v3/contents/generations/tasks 只返回 {"id": "<task_id>"}
	if strings.HasPrefix(c.Request.RequestURI, "/api/v3/contents/generations/tasks") {
		c.JSON(http.StatusOK, gin.H{"id": info.PublicTaskID})
		return dResp.ID, responseBody, nil
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return dResp.ID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	// Add images if present (OpenAI 格式)
	if req.HasImage() {
		for _, imgURL := range req.Images {
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &MediaURL{
					URL: imgURL,
				},
			})
		}
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// 顶层 content：火山原生格式客户端直接提交 content 数组
	if len(r.Content) == 0 && len(req.Content) > 0 {
		var items []ContentItem
		if err := common.Unmarshal(req.Content, &items); err == nil {
			r.Content = items
		}
	}

	// 顶层火山字段兜底：优先取 metadata 中的值，缺失时取顶层字段
	if r.Resolution == "" {
		r.Resolution = req.Resolution
	}
	if r.Ratio == "" {
		r.Ratio = req.Ratio
	}
	if r.Duration == nil && req.Duration > 0 {
		r.Duration = lo.ToPtr(dto.IntValue(req.Duration))
	}
	if r.Frames == nil && req.Frames > 0 {
		r.Frames = lo.ToPtr(dto.IntValue(req.Frames))
	}
	if r.Seed == nil && req.Seed != nil {
		r.Seed = lo.ToPtr(dto.IntValue(*req.Seed))
	}
	if r.GenerateAudio == nil && req.GenerateAudio != nil {
		r.GenerateAudio = lo.ToPtr(dto.BoolValue(*req.GenerateAudio))
	}
	if r.Watermark == nil && req.Watermark != nil {
		r.Watermark = lo.ToPtr(dto.BoolValue(*req.Watermark))
	}
	if r.CameraFixed == nil && req.CameraFixed != nil {
		r.CameraFixed = lo.ToPtr(dto.BoolValue(*req.CameraFixed))
	}
	if r.ReturnLastFrame == nil && req.ReturnLastFrame != nil {
		r.ReturnLastFrame = lo.ToPtr(dto.BoolValue(*req.ReturnLastFrame))
	}
	if r.CallbackURL == "" {
		r.CallbackURL = req.CallbackURL
	}
	if r.ServiceTier == "" {
		r.ServiceTier = req.ServiceTier
	}
	if r.Draft == nil && req.Draft != nil {
		r.Draft = lo.ToPtr(dto.BoolValue(*req.Draft))
	}
	if r.ExecutionExpiresAfter == nil && req.ExecutionExpiresAfter != nil {
		r.ExecutionExpiresAfter = lo.ToPtr(dto.IntValue(*req.ExecutionExpiresAfter))
	}
	if len(r.Tools) == 0 && len(req.Tools) > 0 {
		var tools []struct {
			Type string `json:"type,omitempty"`
		}
		if err := common.Unmarshal(req.Tools, &tools); err == nil {
			r.Tools = tools
		}
	}

	// OpenAI /v1/videos 格式（size / seconds）映射到火山字段
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
	if r.Duration == nil {
		if sec, _ := strconv.Atoi(req.Seconds); sec > 0 {
			r.Duration = lo.ToPtr(dto.IntValue(sec))
		}
	}

	// 兜底：如果 content 里没有文本项，把 prompt 塞进去
	hasText := false
	for _, item := range r.Content {
		if item.Type == "text" {
			hasText = true
			break
		}
	}
	if !hasText && req.Prompt != "" {
		r.Content = append(r.Content, ContentItem{
			Type: "text",
			Text: req.Prompt,
		})
	}

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// Map Doubao status to internal status
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = resTask.Content.VideoURL
		// 解析 usage 信息用于按倍率计费
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Error.Message
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", dResp.Content.VideoURL)
	if dResp.Content.LastFrameURL != "" {
		openAIVideo.SetMetadata("last_frame_url", dResp.Content.LastFrameURL)
	}
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if dResp.Status == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: dResp.Error.Message,
			Code:    dResp.Error.Code,
		}
	}

	return common.Marshal(openAIVideo)
}

// ConvertToVolcVideo 输出火山原生响应体（GET /api/v3/contents/generations/tasks/{id}）。
// 用本地存的 originTask.Data（上游返回的响应体）并覆盖 id 为本地 TaskID，
// 状态字段按火山官方枚举（queued/running/succeeded/failed）输出。
func (a *TaskAdaptor) ConvertToVolcVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if len(originTask.Data) > 0 {
		if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
			return nil, errors.Wrap(err, "unmarshal doubao task data failed")
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
