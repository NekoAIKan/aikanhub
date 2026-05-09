package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestBuildVideoBillingPricingMissingProfileReturnsNil(t *testing.T) {
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"video_billing_setting.profiles": `{}`,
	}))
	require.Nil(t, buildVideoBillingPricing("not-configured"))
}

func TestBuildVideoBillingPricingNonFormulaProfileReturnsNil(t *testing.T) {
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"video_billing_setting.profiles": `{"some-model": {"mode":"per_call","unit_price":1,"fallback_fps":24,"fallback_width":1280,"fallback_height":720,"fallback_duration_seconds":5}}`,
	}))
	require.Nil(t, buildVideoBillingPricing("some-model"))
}

// Headline + matrix should reproduce the same numbers as the
// service-level matrix test (TestSeedanceFixturePricingMatrix) for
// 720p / 5s text and the floor case. Any drift here means the public
// /pricing display has fallen out of sync with the runtime billing.
func TestBuildVideoBillingPricingSeedanceMatrixHeadline(t *testing.T) {
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.upstream_cost_per_million_tokens": "0",
		"profit_setting.default_markup_percent":           "0",
		"profit_setting.apply_to_default_video_profiles":  "false",
		"video_billing_setting.profiles": `{
			"doubao-seedance-2-0-260128": {
				"mode": "formula",
				"unit_price": 7.89,
				"unit_price_with_video": 4.80,
				"unit_price_by_resolution": {"480p": 7.89, "720p": 7.89, "1080p": 8.74},
				"unit_price_with_video_by_resolution": {"480p": 4.80, "720p": 4.80, "1080p": 5.31},
				"min_tokens_with_video": 108000,
				"fallback_fps": 24,
				"fallback_width": 1280,
				"fallback_height": 720,
				"fallback_duration_seconds": 5,
				"use_upstream_usage": true,
				"resolution_aliases": {
					"480p":  {"width": 832,  "height": 480},
					"720p":  {"width": 1280, "height": 720},
					"1080p": {"width": 1920, "height": 1080}
				}
			}
		}`,
	}))

	got := buildVideoBillingPricing("doubao-seedance-2-0-260128")
	require.NotNil(t, got)

	// Headline is fixed at 720p / 5s text.
	require.Equal(t, "720p", got.Headline.Resolution)
	require.Equal(t, 5, got.Headline.DurationSeconds)
	require.False(t, got.Headline.HasVideoInput)
	require.Equal(t, 108000, got.Headline.Tokens)
	require.Equal(t, 7.89, got.Headline.UnitPricePerMillion)
	require.InDelta(t, 0.85212, got.Headline.PriceUSD, 1e-6)
	require.Equal(t, 426060, got.Headline.Quota)

	// Matrix is 3 resolutions × 3 durations × 2 (text/with-video) = 18 cells.
	require.Len(t, got.Matrix, 18)

	// Spot-check a few representative cells against the canonical numbers
	// asserted in service.TestSeedanceFixturePricingMatrix.
	type key struct {
		Res     string
		Dur     int
		HasVideo bool
	}
	byKey := map[key]VideoBillingPricingPoint{}
	for _, c := range got.Matrix {
		byKey[key{c.Resolution, c.DurationSeconds, c.HasVideoInput}] = c
	}
	require.Equal(t, 426060, byKey[key{"720p", 5, false}].Quota)
	require.Equal(t, 1061910, byKey[key{"1080p", 5, false}].Quota)
	require.Equal(t, 518400, byKey[key{"720p", 5, true}].Quota)
	require.Equal(t, 1290330, byKey[key{"1080p", 5, true}].Quota)

	// Rules surface based on profile shape.
	require.Contains(t, got.Rules[0], "Per-token billing")
	require.Contains(t, got.Rules, "Reference video inputs price differently from text/image inputs")
	require.Contains(t, got.Rules, "Reference video requests are subject to a minimum token floor")
	require.Equal(t, "Failed tasks are not billed", got.Rules[len(got.Rules)-1])
}
