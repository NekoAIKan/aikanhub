package service

import (
	"crypto/sha256"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/profit_setting"
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
	HasReferenceMedia   bool
}

type VideoBillingResult struct {
	Tokens            int
	RawCost           float64
	Quota             int
	Basis             string
	InputSeconds      int
	OutputSeconds     int
	Width             int
	Height            int
	FPS               int
	HasReferenceMedia bool
	RetailUnitPrice   float64
	UpstreamUnitCost  float64
	MarkupPercent     float64
	PricingHash       string
	PricingVersion    string
}

func CalculateVideoBilling(profile videobilling.VideoBillingProfile, input VideoBillingInput, preCharge bool) VideoBillingResult {
	normalized := normalizeVideoBillingInput(profile, input)
	basis := VideoBillingBasisFormula

	tokens := ((normalized.InputSeconds + normalized.OutputSeconds) * normalized.Width * normalized.Height * normalized.FPS) / 1024
	if normalized.UpstreamTotalTokens > 0 && profile.UseUpstreamUsage {
		tokens = normalized.UpstreamTotalTokens
		basis = VideoBillingBasisUpstreamUsage
	}

	retailUnitPrice, upstreamUnitCost, markupPercent := resolveVideoRetailUnitPrice(profile)
	rawCost := float64(tokens) * retailUnitPrice
	if normalized.Draft && profile.DraftMultiplier > 0 {
		rawCost *= profile.DraftMultiplier
	}
	if preCharge {
		multiplier := profile.ConservativeMultiplier
		if normalized.HasReferenceMedia && profile.ReferenceConservativeMultiplier > 0 {
			multiplier = profile.ReferenceConservativeMultiplier
		}
		if multiplier > 0 {
			rawCost *= multiplier
		}
	}

	quota := int(rawCost / 1_000_000 * common.QuotaPerUnit * normalized.GroupRatio)
	pricingHash := videoPricingHash(profile, retailUnitPrice, upstreamUnitCost, markupPercent)
	return VideoBillingResult{
		Tokens:            tokens,
		RawCost:           rawCost,
		Quota:             quota,
		Basis:             basis,
		InputSeconds:      normalized.InputSeconds,
		OutputSeconds:     normalized.OutputSeconds,
		Width:             normalized.Width,
		Height:            normalized.Height,
		FPS:               normalized.FPS,
		HasReferenceMedia: normalized.HasReferenceMedia,
		RetailUnitPrice:   retailUnitPrice,
		UpstreamUnitCost:  upstreamUnitCost,
		MarkupPercent:     markupPercent,
		PricingHash:       pricingHash,
		PricingVersion:    "video_formula:v1:" + pricingHash,
	}
}

func resolveVideoRetailUnitPrice(profile videobilling.VideoBillingProfile) (retailUnitPrice float64, upstreamUnitCost float64, markupPercent float64) {
	profit := profit_setting.GetProfitSetting()
	upstreamUnitCost = profit.UpstreamCostPerMillionTokens
	if upstreamUnitCost < 0 {
		upstreamUnitCost = 0
	}
	markupPercent = profit.DefaultMarkupPercent

	if profile.UnitPrice > 0 {
		return profile.UnitPrice, upstreamUnitCost, markupPercent
	}
	if upstreamUnitCost > 0 && profit.ApplyToDefaultVideoProfiles {
		return upstreamUnitCost * (1 + markupPercent/100), upstreamUnitCost, markupPercent
	}
	return 1, upstreamUnitCost, markupPercent
}

func videoPricingHash(profile videobilling.VideoBillingProfile, retailUnitPrice float64, upstreamUnitCost float64, markupPercent float64) string {
	payload := struct {
		Mode                            string                                  `json:"mode"`
		ProfileUnitPrice                float64                                 `json:"profile_unit_price"`
		RetailUnitPrice                 float64                                 `json:"retail_unit_price"`
		UpstreamUnitCost                float64                                 `json:"upstream_unit_cost"`
		MarkupPercent                   float64                                 `json:"markup_percent"`
		FallbackFPS                     int                                     `json:"fallback_fps"`
		FallbackWidth                   int                                     `json:"fallback_width"`
		FallbackHeight                  int                                     `json:"fallback_height"`
		FallbackDurationSeconds         int                                     `json:"fallback_duration_seconds"`
		UseUpstreamUsage                bool                                    `json:"use_upstream_usage"`
		ConservativeMultiplier          float64                                 `json:"conservative_multiplier"`
		ReferenceConservativeMultiplier float64                                 `json:"reference_conservative_multiplier"`
		DraftMultiplier                 float64                                 `json:"draft_multiplier"`
		ResolutionAliases               map[string]videobilling.VideoResolution `json:"resolution_aliases,omitempty"`
	}{
		Mode:                            profile.Mode,
		ProfileUnitPrice:                profile.UnitPrice,
		RetailUnitPrice:                 retailUnitPrice,
		UpstreamUnitCost:                upstreamUnitCost,
		MarkupPercent:                   markupPercent,
		FallbackFPS:                     profile.FallbackFPS,
		FallbackWidth:                   profile.FallbackWidth,
		FallbackHeight:                  profile.FallbackHeight,
		FallbackDurationSeconds:         profile.FallbackDurationSeconds,
		UseUpstreamUsage:                profile.UseUpstreamUsage,
		ConservativeMultiplier:          profile.ConservativeMultiplier,
		ReferenceConservativeMultiplier: profile.ReferenceConservativeMultiplier,
		DraftMultiplier:                 profile.DraftMultiplier,
		ResolutionAliases:               profile.ResolutionAliases,
	}
	data, _ := common.Marshal(payload)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])[:12]
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
