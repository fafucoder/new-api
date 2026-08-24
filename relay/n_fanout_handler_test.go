package relay

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
)

func ptrInt(v int) *int { return &v }

func TestShouldNFanout(t *testing.T) {
	newInfo := func(enabled bool) *relaycommon.RelayInfo {
		// ChannelSetting 是内嵌 *ChannelMeta 提升的字段，需显式初始化 ChannelMeta
		// （生产路径由 TextHelper 的 InitChannelMeta 保证非 nil）。
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelSetting: dto.ChannelSettings{NFanoutEnabled: enabled},
			},
		}
		return info
	}

	tests := []struct {
		name    string
		info    *relaycommon.RelayInfo
		request *dto.GeneralOpenAIRequest
		want    bool
	}{
		{
			name:    "enabled and n>1 fans out",
			info:    newInfo(true),
			request: &dto.GeneralOpenAIRequest{N: ptrInt(3)},
			want:    true,
		},
		{
			name:    "enabled but n==1 does not fan out",
			info:    newInfo(true),
			request: &dto.GeneralOpenAIRequest{N: ptrInt(1)},
			want:    false,
		},
		{
			name:    "enabled but n nil does not fan out",
			info:    newInfo(true),
			request: &dto.GeneralOpenAIRequest{N: nil},
			want:    false,
		},
		{
			name:    "disabled with n>1 does not fan out",
			info:    newInfo(false),
			request: &dto.GeneralOpenAIRequest{N: ptrInt(5)},
			want:    false,
		},
		{
			name:    "nil info",
			info:    nil,
			request: &dto.GeneralOpenAIRequest{N: ptrInt(3)},
			want:    false,
		},
		{
			name:    "nil request",
			info:    newInfo(true),
			request: nil,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldNFanout(tt.info, tt.request))
		})
	}
}

func TestAccumulateUsage(t *testing.T) {
	dst := &dto.Usage{}
	src := &dto.Usage{
		PromptTokens:         10,
		CompletionTokens:     20,
		TotalTokens:          30,
		PromptCacheHitTokens: 4,
		InputTokens:          10,
		OutputTokens:         20,
	}
	src.PromptTokensDetails.CachedTokens = 4
	src.PromptTokensDetails.TextTokens = 6
	src.CompletionTokenDetails.ReasoningTokens = 5
	src.CompletionTokenDetails.TextTokens = 15

	// 累加 3 份（模拟 n=3 fan-out 的用量合并）
	for range 3 {
		accumulateUsage(dst, src)
	}

	assert.Equal(t, 30, dst.PromptTokens)
	assert.Equal(t, 60, dst.CompletionTokens)
	assert.Equal(t, 90, dst.TotalTokens)
	assert.Equal(t, 12, dst.PromptCacheHitTokens)
	assert.Equal(t, 30, dst.InputTokens)
	assert.Equal(t, 60, dst.OutputTokens)
	assert.Equal(t, 12, dst.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 18, dst.PromptTokensDetails.TextTokens)
	assert.Equal(t, 15, dst.CompletionTokenDetails.ReasoningTokens)
	assert.Equal(t, 45, dst.CompletionTokenDetails.TextTokens)
}

func TestAccumulateUsageNilSafe(t *testing.T) {
	// nil dst/src 不应 panic
	assert.NotPanics(t, func() {
		accumulateUsage(nil, &dto.Usage{PromptTokens: 1})
		accumulateUsage(&dto.Usage{}, nil)
		accumulateUsage(nil, nil)
	})
}
