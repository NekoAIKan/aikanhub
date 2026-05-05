package video_billing_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestLoadProfilesFromDBAndReturnDefensiveCopy(t *testing.T) {
	profilesJSON := `{
		"seedance-custom": {
			"mode": "formula",
			"unit_price": 2.5,
			"fallback_fps": 24,
			"fallback_width": 1280,
			"fallback_height": 720,
			"fallback_duration_seconds": 5,
			"use_upstream_usage": true,
			"conservative_multiplier": 1.25,
			"draft_multiplier": 0.5,
			"resolution_aliases": {
				"720p": {"width": 1280, "height": 720},
				"portrait": {"width": 720, "height": 1280}
			}
		}
	}`

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"video_billing_setting.profiles": profilesJSON,
	}))

	profile, ok := GetProfile("seedance-custom")
	require.True(t, ok)
	require.Equal(t, "formula", profile.Mode)
	require.Equal(t, 2.5, profile.UnitPrice)
	require.Equal(t, VideoResolution{Width: 1280, Height: 720}, profile.ResolutionAliases["720p"])

	profile.ResolutionAliases["720p"] = VideoResolution{Width: 1, Height: 1}
	profile.Mode = "mutated"

	again, ok := GetProfile("seedance-custom")
	require.True(t, ok)
	require.Equal(t, "formula", again.Mode)
	require.Equal(t, VideoResolution{Width: 1280, Height: 720}, again.ResolutionAliases["720p"])
}

func TestDefaultSeedanceProfilesExistAndCanBeOverridden(t *testing.T) {
	profile, ok := GetProfile("doubao-seedance-2-0-260128")
	require.True(t, ok)
	require.Equal(t, ModeFormula, profile.Mode)
	require.Positive(t, profile.UnitPrice)
	require.Positive(t, profile.FallbackFPS)
	require.Contains(t, profile.ResolutionAliases, "1080p")

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"video_billing_setting.profiles": `{"doubao-seedance-2-0-260128":{"mode":"formula","unit_price":9,"fallback_fps":30,"fallback_width":640,"fallback_height":360,"fallback_duration_seconds":3,"use_upstream_usage":false,"conservative_multiplier":2,"draft_multiplier":0.25,"resolution_aliases":{"small":{"width":640,"height":360}}}}`,
	}))

	overridden, ok := GetProfile("doubao-seedance-2-0-260128")
	require.True(t, ok)
	require.Equal(t, 9.0, overridden.UnitPrice)
	require.Equal(t, 30, overridden.FallbackFPS)
	require.NotContains(t, overridden.ResolutionAliases, "1080p")
	require.Equal(t, VideoResolution{Width: 640, Height: 360}, overridden.ResolutionAliases["small"])
}
