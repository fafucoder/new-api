package channel_validation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Run executes the configured Step1 → Step2 → Step3 suite against `channel`,
// then aggregates the outcomes into a ValidationResult ready for the API
// response. ctx applies to all upstream calls; ensure callers set a sensible
// deadline (we recommend 3-5 minutes for the full suite).
func Run(ctx context.Context, channel *model.Channel, opts Options) (*ValidationResult, error) {
	if channel == nil {
		return nil, errors.New("channel is nil")
	}
	if strings.TrimSpace(opts.Model) == "" {
		return nil, errors.New("model is required")
	}

	res := &ValidationResult{
		ChannelID:      channel.Id,
		ChannelName:    channel.Name,
		ChannelType:    channel.Type,
		BaseURL:        channel.GetBaseURL(),
		RequestedModel: opts.Model,
		StartedAt:      common.GetTimestamp(),
	}
	suiteStart := time.Now()

	apiKey, _, apiErr := channel.GetNextEnabledKey()
	if apiErr != nil {
		res.Summary = "no usable API key on this channel: " + apiErr.Error()
		res.Verdict = "unknown"
		res.DurationMs = time.Since(suiteStart).Milliseconds()
		return res, nil
	}

	target := buildTargetURL(res.BaseURL)
	client := newHTTPClient()

	// --- Step 1: baseline ---
	step1Body := buildBaselineRequest(opts.Model, opts.MaxTokens, opts.ForcePadding)
	step1Bytes, err := marshalBody(step1Body)
	if err != nil {
		return nil, fmt.Errorf("marshal step1 body: %w", err)
	}
	step1 := performRequest(ctx, client, target, apiKey, step1Bytes, StepKindBaseline)
	step1.RequestedModel = opts.Model
	step1.CachePadApplied = opts.ForcePadding
	res.Steps = append(res.Steps, step1)

	if step1.RespondedModel != "" {
		res.RespondedModel = step1.RespondedModel
	}

	// --- Step 2: multi-turn replay (needs Step1 thinking + signature) ---
	if opts.RunMultiTurn && step1.OK && step1.ThinkingFull != "" && step1.SignatureFull != "" {
		step2Body, err := buildMultiTurnRequest(
			opts.Model, opts.MaxTokens, opts.Step2Prompt,
			step1.ThinkingFull, step1.SignatureFull, step1.OutputTextPreview,
		)
		if err == nil {
			step2Bytes, mErr := marshalBody(step2Body)
			if mErr == nil {
				step2 := performRequest(ctx, client, target, apiKey, step2Bytes, StepKindMultiTurn)
				step2.RequestedModel = opts.Model
				res.Steps = append(res.Steps, step2)
			}
		}
	}

	// --- Step 3: tamper negative test ---
	if opts.RunTamper && step1.OK && step1.SignatureFull != "" {
		tampered := tamperSignature(step1.SignatureFull)
		if tampered != "" {
			step3Body, err := buildMultiTurnRequest(
				opts.Model, opts.MaxTokens, opts.Step2Prompt,
				step1.ThinkingFull, tampered, step1.OutputTextPreview,
			)
			if err == nil {
				step3Bytes, mErr := marshalBody(step3Body)
				if mErr == nil {
					step3 := performRequest(ctx, client, target, apiKey, step3Bytes, StepKindTamper)
					step3.RequestedModel = opts.Model
					res.Steps = append(res.Steps, step3)
				}
			}
		}
	}

	// --- Optional cross-channel verification ---
	if opts.CrossChannelID > 0 && step1.OK && step1.SignatureFull != "" {
		crossOutcome := runCrossCheck(ctx, client, opts, step1)
		if crossOutcome != nil {
			res.Steps = append(res.Steps, *crossOutcome)
		}
	}

	res.Checks = buildChecks(res.Steps)
	res.Verdict, res.OK, res.Summary = computeVerdict(res.Steps, res.Checks)
	res.DurationMs = time.Since(suiteStart).Milliseconds()
	return res, nil
}

// runCrossCheck replays Step2's request structure against a different
// channel (presumably a known-real Claude). If the cross channel accepts
// the signed thinking, the signature is genuine; this builds confidence
// even when the primary channel passes its own self-replay.
func runCrossCheck(ctx context.Context, client *http.Client, opts Options, baseline StepOutcome) *StepOutcome {
	crossChannel, err := model.GetChannelById(opts.CrossChannelID, false)
	if err != nil || crossChannel == nil {
		return nil
	}
	crossKey, _, apiErr := crossChannel.GetNextEnabledKey()
	if apiErr != nil {
		return nil
	}
	crossTarget := buildTargetURL(crossChannel.GetBaseURL())
	step2Body, err := buildMultiTurnRequest(
		opts.Model, opts.MaxTokens, opts.Step2Prompt,
		baseline.ThinkingFull, baseline.SignatureFull, baseline.OutputTextPreview,
	)
	if err != nil {
		return nil
	}
	bodyBytes, mErr := marshalBody(step2Body)
	if mErr != nil {
		return nil
	}
	out := performRequest(ctx, client, crossTarget, crossKey, bodyBytes, StepKindCrossCheck)
	out.RequestedModel = opts.Model
	return &out
}
