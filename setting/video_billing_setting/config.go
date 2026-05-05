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

type VideoBillingProfile struct {
	Mode                    string                     `json:"mode"`
	UnitPrice               float64                    `json:"unit_price"`
	FallbackFPS             int                        `json:"fallback_fps"`
	FallbackWidth           int                        `json:"fallback_width"`
	FallbackHeight          int                        `json:"fallback_height"`
	FallbackDurationSeconds int                        `json:"fallback_duration_seconds"`
	UseUpstreamUsage        bool                       `json:"use_upstream_usage"`
	ConservativeMultiplier  float64                    `json:"conservative_multiplier"`
	DraftMultiplier         float64                    `json:"draft_multiplier"`
	ResolutionAliases       map[string]VideoResolution `json:"resolution_aliases"`
}

type VideoBillingSetting struct {
	Profiles map[string]VideoBillingProfile `json:"profiles"`
}

var videoBillingSetting = VideoBillingSetting{
	Profiles: map[string]VideoBillingProfile{},
}

var defaultProfiles = map[string]VideoBillingProfile{
	"doubao-seedance-1-0-pro-250528":  defaultSeedanceProfile(),
	"doubao-seedance-1-0-lite-t2v":    defaultSeedanceProfile(),
	"doubao-seedance-1-0-lite-i2v":    defaultSeedanceProfile(),
	"doubao-seedance-1-5-pro-251215":  defaultSeedanceProfile(),
	"doubao-seedance-2-0-260128":      defaultSeedanceProfile(),
	"doubao-seedance-2-0-fast-260128": defaultSeedanceProfile(),
}

func init() {
	config.GlobalConfig.Register("video_billing_setting", &videoBillingSetting)
}

func defaultSeedanceProfile() VideoBillingProfile {
	return VideoBillingProfile{
		Mode:                    ModeFormula,
		UnitPrice:               1,
		FallbackFPS:             24,
		FallbackWidth:           1280,
		FallbackHeight:          720,
		FallbackDurationSeconds: 5,
		UseUpstreamUsage:        true,
		ConservativeMultiplier:  1.25,
		DraftMultiplier:         0.5,
		ResolutionAliases: map[string]VideoResolution{
			"480p":      {Width: 832, Height: 480},
			"720p":      {Width: 1280, Height: 720},
			"1080p":     {Width: 1920, Height: 1080},
			"1280x720":  {Width: 1280, Height: 720},
			"720x1280":  {Width: 720, Height: 1280},
			"1920x1080": {Width: 1920, Height: 1080},
			"1080x1920": {Width: 1080, Height: 1920},
		},
	}
}

func GetProfile(model string) (VideoBillingProfile, bool) {
	if profile, ok := videoBillingSetting.Profiles[model]; ok {
		return cloneProfile(profile), true
	}
	profile, ok := defaultProfiles[model]
	if !ok {
		return VideoBillingProfile{}, false
	}
	return cloneProfile(profile), true
}

func cloneProfile(profile VideoBillingProfile) VideoBillingProfile {
	profile.ResolutionAliases = lo.Assign(profile.ResolutionAliases)
	return profile
}
