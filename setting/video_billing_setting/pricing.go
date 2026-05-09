package video_billing_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/profit_setting"
)

// ResolveRetailUnitPrice picks the unit price (USD per 1M tokens) for a
// (profile, has-reference-media, resolution) tuple. Lookup precedence
// (first match wins):
//
//  1. has_reference_media → profile.UnitPriceWithVideoByResolution[resolution]
//  2. has_reference_media → profile.UnitPriceWithVideo
//  3. profile.UnitPriceByResolution[resolution]
//  4. profile.UnitPrice
//  5. profit_setting.upstream_cost_per_million_tokens × (1 + markup%/100)
//     when ApplyToDefaultVideoProfiles is set.
//  6. 0 — request bills free.
//
// Step 6 is intentional: an OSS binary should never silently bill at a
// magic-number fallback. Callers that want a hard error should compare
// the returned retailUnitPrice against zero.
//
// Lives in this package (rather than `service`) so both the runtime
// billing path and the user-facing /pricing controller can call it
// without a circular import.
func ResolveRetailUnitPrice(profile VideoBillingProfile, hasReferenceMedia bool, resolution string) (retailUnitPrice float64, upstreamUnitCost float64, markupPercent float64) {
	profit := profit_setting.GetProfitSetting()
	upstreamUnitCost = profit.UpstreamCostPerMillionTokens
	if upstreamUnitCost < 0 {
		upstreamUnitCost = 0
	}
	markupPercent = profit.DefaultMarkupPercent

	res := strings.TrimSpace(strings.ToLower(resolution))

	if hasReferenceMedia {
		if price, ok := lookupResolutionPrice(profile.UnitPriceWithVideoByResolution, res); ok {
			return price, upstreamUnitCost, markupPercent
		}
		if profile.UnitPriceWithVideo > 0 {
			return profile.UnitPriceWithVideo, upstreamUnitCost, markupPercent
		}
	}
	if price, ok := lookupResolutionPrice(profile.UnitPriceByResolution, res); ok {
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
