package maas

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
)

func TestConvertToRequestPayload_VolcNativeFormat(t *testing.T) {
	a := &TaskAdaptor{}
	req := &relaycommon.TaskSubmitReq{
		Prompt:     "hello",
		Resolution: "1080p",
		Ratio:      "16:9",
		Duration:   10,
	}
	body := a.convertToRequestPayload(req, "doubao-seedance-2-0-260128")

	assert.Equal(t, "1080p", body.Resolution)
	assert.Equal(t, "16:9", body.Ratio)
	if assert.NotNil(t, body.Duration) {
		assert.Equal(t, 10, *body.Duration)
	}
	// prompt 兜底放到 content[0]
	if assert.Len(t, body.Content, 1) {
		assert.Equal(t, "text", body.Content[0].Type)
		assert.Equal(t, "hello", body.Content[0].Text)
	}
}

func TestConvertToRequestPayload_OpenAIFormat(t *testing.T) {
	a := &TaskAdaptor{}
	req := &relaycommon.TaskSubmitReq{
		Prompt:  "hello",
		Size:    "1280x720",
		Seconds: "5",
	}
	body := a.convertToRequestPayload(req, "doubao-seedance-2-0-260128")

	assert.Equal(t, "720p", body.Resolution)
	assert.Equal(t, "16:9", body.Ratio)
	if assert.NotNil(t, body.Duration) {
		assert.Equal(t, 5, *body.Duration)
	}
}

func TestConvertToRequestPayload_OpenAIFormat_Portrait(t *testing.T) {
	a := &TaskAdaptor{}
	req := &relaycommon.TaskSubmitReq{
		Prompt: "hello",
		Size:   "720x1280",
	}
	body := a.convertToRequestPayload(req, "doubao-seedance-2-0-260128")

	assert.Equal(t, "720p", body.Resolution)
	assert.Equal(t, "9:16", body.Ratio)
}

func TestConvertToRequestPayload_MetadataOverridesTopLevel(t *testing.T) {
	a := &TaskAdaptor{}
	req := &relaycommon.TaskSubmitReq{
		Prompt:     "hello",
		Resolution: "480p",
		Metadata: map[string]interface{}{
			"resolution": "1080p",
			"ratio":      "9:16",
		},
	}
	body := a.convertToRequestPayload(req, "doubao-seedance-2-0-260128")

	// metadata 已被 UnmarshalMetadata 写入 r.Resolution 为 1080p，随后顶层的 480p 不覆盖
	assert.Equal(t, "1080p", body.Resolution)
	assert.Equal(t, "9:16", body.Ratio)
}

func TestParseOpenAISize(t *testing.T) {
	cases := []struct {
		size    string
		wantRes string
		wantR   string
		wantOK  bool
	}{
		{"1280x720", "720p", "16:9", true},
		{"720x1280", "720p", "9:16", true},
		{"1920x1080", "1080p", "16:9", true},
		{"480x480", "480p", "1:1", true},
		{"3840x2160", "4k", "16:9", true},
		{"", "", "", false},
		{"invalid", "", "", false},
		{"1280xabc", "", "", false},
	}
	for _, c := range cases {
		res, r, ok := parseOpenAISize(c.size)
		assert.Equal(t, c.wantOK, ok, c.size)
		if c.wantOK {
			assert.Equal(t, c.wantRes, res, c.size)
			assert.Equal(t, c.wantR, r, c.size)
		}
	}
}
