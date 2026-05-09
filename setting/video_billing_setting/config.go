package video_billing_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/samber/lo"
)

const (
	ModeFormula = "formula"
)

type VideoResolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// VideoBillingProfile is the per-model billing recipe for a video task.
// All pricing fields are admin-configured at runtime via the
// `video_billing_setting.profiles` option; nothing in this repo ships
// vendor-specific prices.
type VideoBillingProfile struct {
	Mode      string  `json:"mode"`
	UnitPrice float64 `json:"unit_price"`

	// UnitPriceWithVideo overrides UnitPrice when the request carries video
	// reference media. Vendors typically charge less when the input is a
	// video clip rather than text/image; admins capture that gap here.
	UnitPriceWithVideo float64 `json:"unit_price_with_video,omitempty"`

	// UnitPriceByResolution lets admins price each output resolution
	// distinctly. Keys match the `Resolution` alias surfaced by the request
	// (e.g. "720p", "1080p"); no entry → fall back to UnitPrice.
	UnitPriceByResolution map[string]float64 `json:"unit_price_by_resolution,omitempty"`

	// UnitPriceWithVideoByResolution is the with-video variant of
	// UnitPriceByResolution; resolved before UnitPriceByResolution when the
	// request has reference media.
	UnitPriceWithVideoByResolution map[string]float64 `json:"unit_price_with_video_by_resolution,omitempty"`

	// MinTokensWithVideo is a token floor applied only when reference media
	// is present and the formula-derived (or upstream-reported) token count
	// falls below it. Mirrors vendors that set per-call minimums on
	// video-input requests.
	MinTokensWithVideo int `json:"min_tokens_with_video,omitempty"`

	FallbackFPS                     int                        `json:"fallback_fps"`
	FallbackWidth                   int                        `json:"fallback_width"`
	FallbackHeight                  int                        `json:"fallback_height"`
	FallbackDurationSeconds         int                        `json:"fallback_duration_seconds"`
	UseUpstreamUsage                bool                       `json:"use_upstream_usage"`
	ConservativeMultiplier          float64                    `json:"conservative_multiplier"`
	ReferenceConservativeMultiplier float64                    `json:"reference_conservative_multiplier"`
	DraftMultiplier                 float64                    `json:"draft_multiplier"`
	ResolutionAliases               map[string]VideoResolution `json:"resolution_aliases"`
}

type VideoBillingSetting struct {
	Profiles map[string]VideoBillingProfile `json:"profiles"`
}

var videoBillingSetting = VideoBillingSetting{
	Profiles: map[string]VideoBillingProfile{},
}

func init() {
	config.GlobalConfig.Register("video_billing_setting", &videoBillingSetting)
}

// GetProfile returns the admin-configured profile for `model`, or false if
// none is registered. The repo intentionally ships no defaults: all
// per-model billing parameters must be set at runtime by the operator.
func GetProfile(model string) (VideoBillingProfile, bool) {
	profile, ok := videoBillingSetting.Profiles[model]
	if !ok {
		return VideoBillingProfile{}, false
	}
	return cloneProfile(profile), true
}

// ListConfiguredModels returns the model keys currently present in
// video_billing_setting.profiles. Order is not guaranteed; callers that
// need deterministic order should sort.
func ListConfiguredModels() []string {
	keys := make([]string, 0, len(videoBillingSetting.Profiles))
	for k := range videoBillingSetting.Profiles {
		keys = append(keys, k)
	}
	return keys
}

func cloneProfile(profile VideoBillingProfile) VideoBillingProfile {
	profile.ResolutionAliases = lo.Assign(profile.ResolutionAliases)
	profile.UnitPriceByResolution = lo.Assign(profile.UnitPriceByResolution)
	profile.UnitPriceWithVideoByResolution = lo.Assign(profile.UnitPriceWithVideoByResolution)
	return profile
}
