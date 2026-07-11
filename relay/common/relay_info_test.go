package common

import (
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}

func TestGetClientFacingModelName(t *testing.T) {
	// 开关关闭：返回空串（不改写）
	off := &RelayInfo{OriginModelName: "GLM5.2", ChannelMeta: &ChannelMeta{}}
	require.Equal(t, "", off.GetClientFacingModelName())

	// 开关开启：返回请求的原始模型名
	on := &RelayInfo{OriginModelName: "GLM5.2", ChannelMeta: &ChannelMeta{}}
	on.ChannelSetting.UnifyModelName = true
	require.Equal(t, "GLM5.2", on.GetClientFacingModelName())

	// 开关开启但 OriginModelName 为空：返回空串
	empty := &RelayInfo{ChannelMeta: &ChannelMeta{}}
	empty.ChannelSetting.UnifyModelName = true
	require.Equal(t, "", empty.GetClientFacingModelName())

	// ChannelMeta 为 nil 时安全（不 panic）
	noMeta := &RelayInfo{OriginModelName: "GLM5.2"}
	require.Equal(t, "", noMeta.GetClientFacingModelName())

	// nil receiver 安全
	var nilInfo *RelayInfo
	require.Equal(t, "", nilInfo.GetClientFacingModelName())
}
