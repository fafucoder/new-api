package maas

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Seedance token 估算说明：
// 火山官方公式：tokens = ⌈ width × height × fps × duration / 1024 ⌉，Seedance fps = 24。
// 抵扣口径：以"含视频输入"为 1:1 基准；"无视频输入"场景按 92/56 ≈ 1.6429 折算多扣资源包额度。
const (
	seedanceFPS              = 24
	seedanceTokenDivisor     = 1024
	seedanceNoVideoInputRate = 1.6429 // 无视频输入场景的抵扣倍率
	seedanceDefaultDuration  = 5      // 文档默认时长（秒）
	seedanceDefaultRatio     = "16:9"
)

// resolutionRatioDimensions 按 resolution + ratio 查表得 (width, height)。
// 表值与火山 Seedance 实际输出像素对齐（短边等于 resolution 数值，长边按比例并对齐到偶数）。
var resolutionRatioDimensions = map[string]map[string][2]int{
	"480p": {
		"1:1":  {480, 480},
		"16:9": {864, 480},
		"9:16": {480, 864},
		"4:3":  {640, 480},
		"3:4":  {480, 640},
		"21:9": {1120, 480},
		"9:21": {480, 1120},
	},
	"720p": {
		"1:1":  {720, 720},
		"16:9": {1280, 720},
		"9:16": {720, 1280},
		"4:3":  {960, 720},
		"3:4":  {720, 960},
		"21:9": {1680, 720},
		"9:21": {720, 1680},
	},
	"1080p": {
		"1:1":  {1080, 1080},
		"16:9": {1920, 1080},
		"9:16": {1080, 1920},
		"4:3":  {1440, 1080},
		"3:4":  {1080, 1440},
		"21:9": {2520, 1080},
		"9:21": {1080, 2520},
	},
}

// resolveDimensions 按 resolution + ratio 查表得 (width, height)。
// 兜底：resolution 缺失/未识别 → 720p；ratio 缺失/未识别 → 16:9。
func resolveDimensions(resolution, ratio string) (int, int) {
	resKey := strings.ToLower(strings.TrimSpace(resolution))
	if _, ok := resolutionRatioDimensions[resKey]; !ok {
		resKey = "720p"
	}
	ratioKey := strings.TrimSpace(ratio)
	if _, ok := resolutionRatioDimensions[resKey][ratioKey]; !ok {
		ratioKey = seedanceDefaultRatio
	}
	dim := resolutionRatioDimensions[resKey][ratioKey]
	return dim[0], dim[1]
}

// estimateSeedanceTokens 计算单次任务的 token 数。
// hasVideoInput=true 时不乘倍率；false 时乘 1.6429。
func estimateSeedanceTokens(resolution, ratio string, duration int, hasVideoInput bool) int {
	if duration <= 0 {
		duration = seedanceDefaultDuration
	}
	w, h := resolveDimensions(resolution, ratio)
	raw := math.Ceil(float64(w*h*seedanceFPS*duration) / float64(seedanceTokenDivisor))
	if !hasVideoInput {
		raw = math.Ceil(raw * seedanceNoVideoInputRate)
	}
	return int(raw)
}

// requestSnapshot 提交阶段持久化到 task.PrivateData.RequestSnapshot 的请求参数快照。
// 仅 maas 内部使用，其他 channel 不读取此字段。
type requestSnapshot struct {
	Resolution    string `json:"resolution,omitempty"`
	Ratio         string `json:"ratio,omitempty"`
	Duration      int    `json:"duration,omitempty"`
	HasVideoInput bool   `json:"has_video_input,omitempty"`
}

func encodeRequestSnapshot(s requestSnapshot) ([]byte, error) {
	return common.Marshal(s)
}

func decodeRequestSnapshot(data json.RawMessage) (requestSnapshot, bool) {
	var s requestSnapshot
	if len(data) == 0 {
		return s, false
	}
	if err := common.Unmarshal(data, &s); err != nil {
		return s, false
	}
	return s, true
}
