package service

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/profit_setting"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
)

const (
	VideoBillingBasisFormula       = "formula"
	VideoBillingBasisUpstreamUsage = "upstream_usage"
)

type VideoBillingInput struct {
	InputSeconds  int
	OutputSeconds int
	Width         int
	Height        int
	FPS           int
	// Resolution is the alias the caller used (e.g. "720p", "1080p"); empty
	// when the request only specified raw width/height. Used to look up
	// per-resolution unit prices in the profile.
	Resolution          string
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
	Resolution        string
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
	// Vendors with reference-media inputs (e.g. Volcano Seedance video-edit)
	// publish minimum-token floors. Apply only when the request actually
	// carries video and the formula/upstream value is below the floor.
	if normalized.HasReferenceMedia && profile.MinTokensWithVideo > 0 && tokens < profile.MinTokensWithVideo {
		tokens = profile.MinTokensWithVideo
	}

	retailUnitPrice, upstreamUnitCost, markupPercent := resolveVideoRetailUnitPrice(profile, normalized.HasReferenceMedia, normalized.Resolution)
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
		Resolution:        normalized.Resolution,
		HasReferenceMedia: normalized.HasReferenceMedia,
		RetailUnitPrice:   retailUnitPrice,
		UpstreamUnitCost:  upstreamUnitCost,
		MarkupPercent:     markupPercent,
		PricingHash:       pricingHash,
		PricingVersion:    "video_formula:v1:" + pricingHash,
	}
}

// resolveVideoRetailUnitPrice picks the unit price (USD per 1M tokens) the
// caller's request resolves to. Lookup precedence (first match wins):
//
//  1. Request has reference media → UnitPriceWithVideoByResolution[resolution]
//  2. Request has reference media → UnitPriceWithVideo
//  3. UnitPriceByResolution[resolution]
//  4. UnitPrice
//  5. profit_setting.upstream_cost_per_million_tokens × (1 + markup%) when
//     ApplyToDefaultVideoProfiles is set
//  6. 0 — the request is treated as free.
//
// Step 6 is intentionally not a magic-number fallback ($1/M was historical):
// shipping a non-zero default would bake vendor-agnostic pricing into the
// open-source binary, so we return zero and force the operator to configure
// either the per-profile fields or the global profit_setting.
func resolveVideoRetailUnitPrice(profile videobilling.VideoBillingProfile, hasReferenceMedia bool, resolution string) (retailUnitPrice float64, upstreamUnitCost float64, markupPercent float64) {
	profit := profit_setting.GetProfitSetting()
	upstreamUnitCost = profit.UpstreamCostPerMillionTokens
	if upstreamUnitCost < 0 {
		upstreamUnitCost = 0
	}
	markupPercent = profit.DefaultMarkupPercent

	if hasReferenceMedia {
		if price, ok := lookupResolutionPrice(profile.UnitPriceWithVideoByResolution, resolution); ok {
			return price, upstreamUnitCost, markupPercent
		}
		if profile.UnitPriceWithVideo > 0 {
			return profile.UnitPriceWithVideo, upstreamUnitCost, markupPercent
		}
	}
	if price, ok := lookupResolutionPrice(profile.UnitPriceByResolution, resolution); ok {
		return price, upstreamUnitCost, markupPercent
	}
	if profile.UnitPrice > 0 {
		return profile.UnitPrice, upstreamUnitCost, markupPercent
	}
	if upstreamUnitCost > 0 && profit.ApplyToDefaultVideoProfiles {
		return upstreamUnitCost * (1 + markupPercent/100), upstreamUnitCost, markupPercent
	}
	return 0, upstreamUnitCost, markupPercent
}

func lookupResolutionPrice(table map[string]float64, resolution string) (float64, bool) {
	if len(table) == 0 || resolution == "" {
		return 0, false
	}
	if price, ok := table[resolution]; ok && price > 0 {
		return price, true
	}
	return 0, false
}

func videoPricingHash(profile videobilling.VideoBillingProfile, retailUnitPrice float64, upstreamUnitCost float64, markupPercent float64) string {
	payload := struct {
		Mode                            string                                  `json:"mode"`
		ProfileUnitPrice                float64                                 `json:"profile_unit_price"`
		ProfileUnitPriceWithVideo       float64                                 `json:"profile_unit_price_with_video,omitempty"`
		ProfileUnitPriceByResolution    map[string]float64                      `json:"profile_unit_price_by_resolution,omitempty"`
		ProfileUnitPriceWithVideoByRes  map[string]float64                      `json:"profile_unit_price_with_video_by_resolution,omitempty"`
		MinTokensWithVideo              int                                     `json:"min_tokens_with_video,omitempty"`
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
		ProfileUnitPriceWithVideo:       profile.UnitPriceWithVideo,
		ProfileUnitPriceByResolution:    profile.UnitPriceByResolution,
		ProfileUnitPriceWithVideoByRes:  profile.UnitPriceWithVideoByResolution,
		MinTokensWithVideo:              profile.MinTokensWithVideo,
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
	input.Resolution = strings.TrimSpace(strings.ToLower(input.Resolution))
	return input
}
