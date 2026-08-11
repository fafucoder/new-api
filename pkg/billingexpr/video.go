package billingexpr

import (
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
)

// VideoRequestInfo is the normalized subset of a task request used by video
// pricing and log display.
type VideoRequestInfo struct {
	Resolution        string
	HasReferenceVideo bool
}

// VideoTierInfo describes the stable tier label emitted by the video pricing
// editors. Default is true when the request used the configured default
// resolution.
type VideoTierInfo struct {
	Resolution        string
	HasReferenceVideo bool
	Default           bool
}

// ParseVideoTierLabel parses labels in the form video|720p|0 or
// video|720p|0|default.
func ParseVideoTierLabel(label string) (VideoTierInfo, bool) {
	parts := strings.Split(label, "|")
	if len(parts) < 3 || len(parts) > 4 || parts[0] != "video" {
		return VideoTierInfo{}, false
	}
	resolution, err := url.PathUnescape(parts[1])
	if err != nil || strings.TrimSpace(resolution) == "" {
		return VideoTierInfo{}, false
	}
	if parts[2] != "0" && parts[2] != "1" {
		return VideoTierInfo{}, false
	}
	isDefault := len(parts) == 4 && parts[3] == "default"
	if len(parts) == 4 && !isDefault {
		return VideoTierInfo{}, false
	}
	return VideoTierInfo{
		Resolution:        strings.ToLower(strings.TrimSpace(resolution)),
		HasReferenceVideo: parts[2] == "1",
		Default:           isDefault,
	}, true
}

// ExtractVideoRequestInfo reads common task request shapes without coupling
// the billing package to a specific video provider.
func ExtractVideoRequestInfo(request RequestInput) (VideoRequestInfo, bool) {
	if len(request.Body) == 0 {
		return VideoRequestInfo{}, false
	}

	info := VideoRequestInfo{}
	for _, path := range []string{"metadata.resolution", "resolution", "metadata.size", "size"} {
		if value := strings.TrimSpace(gjson.GetBytes(request.Body, path).String()); value != "" {
			info.Resolution = strings.ToLower(value)
			break
		}
	}

	for _, path := range []string{"has_reference_video", "metadata.has_reference_video"} {
		value := gjson.GetBytes(request.Body, path)
		if value.Exists() && (value.Type == gjson.True || value.Type == gjson.False) {
			info.HasReferenceVideo = value.Bool()
			return info, true
		}
	}

	for _, path := range []string{"metadata.content", "content"} {
		content := gjson.GetBytes(request.Body, path)
		if !content.IsArray() {
			continue
		}
		for _, item := range content.Array() {
			if item.Get("type").String() == "video_url" || item.Get("video_url").Exists() {
				info.HasReferenceVideo = true
				return info, true
			}
		}
	}

	for _, path := range []string{
		"input_reference",
		"video_url",
		"video",
		"metadata.input_reference",
		"metadata.video_url",
		"metadata.video",
	} {
		value := gjson.GetBytes(request.Body, path)
		if value.Exists() && strings.TrimSpace(value.String()) != "" {
			info.HasReferenceVideo = true
			return info, true
		}
	}

	return info, info.Resolution != ""
}
