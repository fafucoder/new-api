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

// Video billing units. VideoUnitToken prices per 1M completion tokens (legacy
// default); VideoUnitSecond prices per video second read from the request body.
const (
	VideoUnitToken  = "token"
	VideoUnitSecond = "second"
)

// VideoTierInfo describes the stable tier label emitted by the video pricing
// editors. Default is true when the request used the configured default
// resolution. Unit is VideoUnitToken or VideoUnitSecond.
type VideoTierInfo struct {
	Resolution        string
	HasReferenceVideo bool
	Default           bool
	Unit              string
}

// ParseVideoTierLabel parses labels in the following stable forms:
//
//	video|720p|0                — per-token, non-default
//	video|720p|0|default        — per-token, default resolution
//	video|720p|0|s              — per-second, non-default
//	video|720p|0|s|default      — per-second, default resolution
//
// The optional "s" segment marks per-second pricing; its absence means
// per-token (backward compatible with historical labels/logs).
func ParseVideoTierLabel(label string) (VideoTierInfo, bool) {
	parts := strings.Split(label, "|")
	if len(parts) < 3 || len(parts) > 5 || parts[0] != "video" {
		return VideoTierInfo{}, false
	}
	resolution, err := url.PathUnescape(parts[1])
	if err != nil || strings.TrimSpace(resolution) == "" {
		return VideoTierInfo{}, false
	}
	if parts[2] != "0" && parts[2] != "1" {
		return VideoTierInfo{}, false
	}

	unit := VideoUnitToken
	isDefault := false
	// Remaining optional segments must appear in order: ["s"]? then ["default"]?.
	for _, seg := range parts[3:] {
		switch seg {
		case "s":
			if unit == VideoUnitSecond || isDefault {
				return VideoTierInfo{}, false
			}
			unit = VideoUnitSecond
		case "default":
			if isDefault {
				return VideoTierInfo{}, false
			}
			isDefault = true
		default:
			return VideoTierInfo{}, false
		}
	}

	return VideoTierInfo{
		Resolution:        strings.ToLower(strings.TrimSpace(resolution)),
		HasReferenceVideo: parts[2] == "1",
		Default:           isDefault,
		Unit:              unit,
	}, true
}

// ExtractVideoDuration reads the video duration in seconds from common request
// body shapes. Returns the duration and true when a positive value is found.
func ExtractVideoDuration(request RequestInput) (float64, bool) {
	if len(request.Body) == 0 {
		return 0, false
	}
	for _, path := range []string{"duration", "metadata.duration", "seconds", "metadata.seconds"} {
		value := gjson.GetBytes(request.Body, path)
		if !value.Exists() {
			continue
		}
		if seconds := value.Float(); seconds > 0 {
			return seconds, true
		}
	}
	return 0, false
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
