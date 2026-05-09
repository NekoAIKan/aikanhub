package model

import (
	"github.com/QuantumNous/new-api/common"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
)

// canonicalMatrixResolution is one (alias, w, h) tuple in the public
// pricing matrix shipped with each video-formula model. Order is fixed
// (smallest → largest) so the frontend can render rows deterministically.
type canonicalMatrixResolution struct {
	Alias  string
	Width  int
	Height int
}

var canonicalMatrixResolutions = []canonicalMatrixResolution{
	{Alias: "480p", Width: 832, Height: 480},
	{Alias: "720p", Width: 1280, Height: 720},
	{Alias: "1080p", Width: 1920, Height: 1080},
}

// canonicalMatrixDurations is the duration set surfaced on /pricing for
// the at-a-glance matrix. The /pricing/$model details page calls the
// public preview endpoint for arbitrary durations.
var canonicalMatrixDurations = []int{5, 10, 15}

// canonicalHeadlineScenario describes which (resolution, duration,
// has_video_input) cell becomes the model's "from $X" headline price.
// Picked to match the most common entry-level use case (text-to-video
// at the popular resolution / duration), so the headline is meaningful
// across vendors without being misleading.
const (
	headlineDuration   = 5
	headlineResolution = "720p"
)

// buildVideoBillingPricing returns nil when `model` has no formula
// profile registered; otherwise it builds the headline + canonical
// matrix from the same `setting/video_billing_setting` data the
// runtime consumes. Pricing is computed locally (no service call) to
// avoid an import cycle: model → service → model.
func buildVideoBillingPricing(model string) *VideoBillingPricing {
	profile, ok := videobilling.GetProfile(model)
	if !ok || profile.Mode != videobilling.ModeFormula {
		return nil
	}
	fps := profile.FallbackFPS
	if fps <= 0 {
		fps = 24
	}

	matrix := make([]VideoBillingPricingPoint, 0, len(canonicalMatrixResolutions)*len(canonicalMatrixDurations)*2)
	for _, r := range canonicalMatrixResolutions {
		w, h := resolveProfileDimensions(profile, r)
		for _, dur := range canonicalMatrixDurations {
			matrix = append(matrix, computeVideoBillingPoint(profile, r.Alias, w, h, fps, dur, false))
			matrix = append(matrix, computeVideoBillingPoint(profile, r.Alias, w, h, fps, dur, true))
		}
	}

	headlineWidth, headlineHeight := resolveProfileDimensions(profile, canonicalMatrixResolution{Alias: headlineResolution, Width: 1280, Height: 720})
	headline := computeVideoBillingPoint(profile, headlineResolution, headlineWidth, headlineHeight, fps, headlineDuration, false)
	headline.Scenario = headlineResolution + " / " + itoa(headlineDuration) + "s text"

	return &VideoBillingPricing{
		Headline: headline,
		Matrix:   matrix,
		Rules:    videoBillingDisplayRules(profile),
	}
}

// resolveProfileDimensions returns the (width, height) for a canonical
// alias by consulting the profile's `resolution_aliases` map first;
// otherwise it falls back to the hardcoded canonical pixels. The alias
// table is operator-controlled so the system can adapt to vendors that
// publish 720p as e.g. 1280×704 instead of 1280×720.
func resolveProfileDimensions(profile videobilling.VideoBillingProfile, r canonicalMatrixResolution) (int, int) {
	if profile.ResolutionAliases != nil {
		if dim, ok := profile.ResolutionAliases[r.Alias]; ok && dim.Width > 0 && dim.Height > 0 {
			return dim.Width, dim.Height
		}
	}
	return r.Width, r.Height
}

// computeVideoBillingPoint applies the same token formula and unit-price
// resolution as `service.CalculateVideoBilling`, minus the conservative
// pre-charge multipliers (which don't belong in a public price list).
// MinTokensWithVideo is honoured because it changes the customer-facing
// minimum bill on with-video requests.
func computeVideoBillingPoint(profile videobilling.VideoBillingProfile, resolution string, w, h, fps, dur int, hasVideo bool) VideoBillingPricingPoint {
	var inputSeconds int
	if hasVideo {
		inputSeconds = dur
	}
	tokens := ((inputSeconds + dur) * w * h * fps) / 1024
	if hasVideo && profile.MinTokensWithVideo > 0 && tokens < profile.MinTokensWithVideo {
		tokens = profile.MinTokensWithVideo
	}
	unitPrice, _, _ := videobilling.ResolveRetailUnitPrice(profile, hasVideo, resolution)
	rawCost := float64(tokens) * unitPrice
	priceUSD := rawCost / 1_000_000
	quota := int(priceUSD * common.QuotaPerUnit)

	return VideoBillingPricingPoint{
		Resolution:          resolution,
		Width:               w,
		Height:              h,
		DurationSeconds:     dur,
		HasVideoInput:       hasVideo,
		Tokens:              tokens,
		UnitPricePerMillion: unitPrice,
		PriceUSD:            priceUSD,
		Quota:               quota,
	}
}

// videoBillingDisplayRules returns short bullet points that explain
// non-obvious billing behaviour to the customer. Generated from the
// profile shape, not hardcoded — so a fork that turns off
// MinTokensWithVideo simply won't render that line.
func videoBillingDisplayRules(profile videobilling.VideoBillingProfile) []string {
	rules := []string{
		"Per-token billing: tokens = (input_seconds + output_seconds) × width × height × fps / 1024",
	}
	if profile.UnitPriceWithVideo > 0 || len(profile.UnitPriceWithVideoByResolution) > 0 {
		rules = append(rules, "Reference video inputs price differently from text/image inputs")
	}
	if profile.MinTokensWithVideo > 0 {
		rules = append(rules, "Reference video requests are subject to a minimum token floor")
	}
	rules = append(rules, "Failed tasks are not billed")
	return rules
}

func itoa(n int) string {
	// Tiny helper to avoid importing strconv just for one call.
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	buf := make([]byte, 0, 6)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if negative {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
