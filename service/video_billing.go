package service

import (
	"github.com/QuantumNous/new-api/common"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
)

const (
	VideoBillingBasisFormula       = "formula"
	VideoBillingBasisUpstreamUsage = "upstream_usage"
)

type VideoBillingInput struct {
	InputSeconds        int
	OutputSeconds       int
	Width               int
	Height              int
	FPS                 int
	GroupRatio          float64
	Draft               bool
	UpstreamTotalTokens int
}

type VideoBillingResult struct {
	Tokens        int
	RawCost       float64
	Quota         int
	Basis         string
	InputSeconds  int
	OutputSeconds int
	Width         int
	Height        int
	FPS           int
}

func CalculateVideoBilling(profile videobilling.VideoBillingProfile, input VideoBillingInput, preCharge bool) VideoBillingResult {
	normalized := normalizeVideoBillingInput(profile, input)
	basis := VideoBillingBasisFormula

	tokens := ((normalized.InputSeconds + normalized.OutputSeconds) * normalized.Width * normalized.Height * normalized.FPS) / 1024
	if normalized.UpstreamTotalTokens > 0 && profile.UseUpstreamUsage {
		tokens = normalized.UpstreamTotalTokens
		basis = VideoBillingBasisUpstreamUsage
	}

	rawCost := float64(tokens) * profile.UnitPrice
	if normalized.Draft && profile.DraftMultiplier > 0 {
		rawCost *= profile.DraftMultiplier
	}
	if preCharge && profile.ConservativeMultiplier > 0 {
		rawCost *= profile.ConservativeMultiplier
	}

	quota := int(rawCost / 1_000_000 * common.QuotaPerUnit * normalized.GroupRatio)
	return VideoBillingResult{
		Tokens:        tokens,
		RawCost:       rawCost,
		Quota:         quota,
		Basis:         basis,
		InputSeconds:  normalized.InputSeconds,
		OutputSeconds: normalized.OutputSeconds,
		Width:         normalized.Width,
		Height:        normalized.Height,
		FPS:           normalized.FPS,
	}
}

func normalizeVideoBillingInput(profile videobilling.VideoBillingProfile, input VideoBillingInput) VideoBillingInput {
	if input.OutputSeconds <= 0 {
		input.OutputSeconds = profile.FallbackDurationSeconds
	}
	if input.Width <= 0 {
		input.Width = profile.FallbackWidth
	}
	if input.Height <= 0 {
		input.Height = profile.FallbackHeight
	}
	if input.FPS <= 0 {
		input.FPS = profile.FallbackFPS
	}
	if input.GroupRatio <= 0 {
		input.GroupRatio = 1
	}
	return input
}
