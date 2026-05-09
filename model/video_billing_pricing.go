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

// canonicalPerSecondResolutions covers the 4-tier resolutions that
// per-second vendors typically expose (Pixverse ships 360p as the
// entry tier; formula vendors usually start at 480p). Order matters
// for matrix rendering — smallest first.
var canonicalPerSecondResolutions = []canonicalMatrixResolution{
	{Alias: "360p", Width: 640, Height: 360},
	{Alias: "540p", Width: 960, Height: 540},
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

// buildVideoBillingPricing returns nil when `model` has no profile
// registered; otherwise it builds the headline + canonical matrix from
// the same `setting/video_billing_setting` data the runtime consumes.
// Pricing is computed locally (no service call) to avoid an import
// cycle: model → service → model.
func buildVideoBillingPricing(model string) *VideoBillingPricing {
	profile, ok := videobilling.GetProfile(model)
	if !ok {
		return nil
	}
	switch profile.Mode {
	case videobilling.ModeFormula:
		return buildFormulaPricing(profile)
	case videobilling.ModePerSecond:
		return buildPerSecondPricing(profile)
	default:
		return nil
	}
}

func buildFormulaPricing(profile videobilling.VideoBillingProfile) *VideoBillingPricing {
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

// buildPerSecondPricing emits a 4 (resolution) × 3 (duration) × 2
// (audio) matrix. Although per_second pricing is linear in duration —
// we don't strictly need to enumerate durations — surfacing them here
// lets the customer compare seedance-style and pixverse-style models
// on the same axes without having to mentally extrapolate.
func buildPerSecondPricing(profile videobilling.VideoBillingProfile) *VideoBillingPricing {
	matrix := make([]VideoBillingPricingPoint, 0, len(canonicalPerSecondResolutions)*len(canonicalMatrixDurations)*2)
	for _, r := range canonicalPerSecondResolutions {
		w, h := resolveProfileDimensions(profile, r)
		for _, dur := range canonicalMatrixDurations {
			matrix = append(matrix, computePerSecondPoint(profile, r.Alias, w, h, dur, false))
			matrix = append(matrix, computePerSecondPoint(profile, r.Alias, w, h, dur, true))
		}
	}

	// Headline: 540p / 5s no-audio is the tier that maps to "$1 = N
	// videos" across vendors that publish that comparison metric.
	const headlineRes = "540p"
	headlineW, headlineH := resolveProfileDimensions(profile, canonicalMatrixResolution{Alias: headlineRes, Width: 960, Height: 540})
	headline := computePerSecondPoint(profile, headlineRes, headlineW, headlineH, headlineDuration, false)
	headline.Scenario = headlineRes + " / " + itoa(headlineDuration) + "s no-audio"

	return &VideoBillingPricing{
		Headline: headline,
		Matrix:   matrix,
		Rules:    perSecondDisplayRules(profile),
	}
}

// computePerSecondPoint mirrors computeVideoBillingPoint but for the
// per_second mode. PriceUSD is the customer-facing total for the cell;
// UnitPricePerMillion is set to 0 (not applicable) and the wire-side
// `price_per_second_usd` carries the rate so the frontend can render it.
func computePerSecondPoint(profile videobilling.VideoBillingProfile, resolution string, w, h, dur int, hasAudio bool) VideoBillingPricingPoint {
	rate, _, _ := videobilling.ResolvePerSecondRate(profile, hasAudio, resolution)
	priceUSD := float64(dur) * rate
	quota := int(priceUSD * common.QuotaPerUnit)
	return VideoBillingPricingPoint{
		Resolution:        resolution,
		Width:             w,
		Height:            h,
		DurationSeconds:   dur,
		HasVideoInput:     false,
		HasAudioInput:     hasAudio,
		Tokens:            0,
		PricePerSecondUSD: rate,
		PriceUSD:          priceUSD,
		Quota:             quota,
	}
}

func perSecondDisplayRules(profile videobilling.VideoBillingProfile) []string {
	rules := []string{
		"Per-second billing: customer pays output_seconds × per-second rate",
	}
	if profile.PricePerSecondWithAudio > 0 || len(profile.PricePerSecondWithAudioByResolution) > 0 {
		rules = append(rules, "Audio output is opt-in and priced separately from video")
	}
	rules = append(rules, "Failed tasks are not billed")
	return rules
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
