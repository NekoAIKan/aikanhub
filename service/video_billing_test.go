package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
	"github.com/stretchr/testify/require"
)

func TestVideoBillingCalculatorFormulaAndQuota(t *testing.T) {
	profile := videobilling.VideoBillingProfile{
		Mode:                    videobilling.ModeFormula,
		UnitPrice:               2,
		FallbackFPS:             24,
		FallbackWidth:           1280,
		FallbackHeight:          720,
		FallbackDurationSeconds: 5,
		UseUpstreamUsage:        true,
		ConservativeMultiplier:  1.25,
		DraftMultiplier:         0.5,
	}

	tests := []struct {
		name        string
		input       VideoBillingInput
		preCharge   bool
		wantTokens  int
		wantRawCost float64
		wantQuota   int
		wantBasis   string
	}{
		{
			name: "720p 5s 24fps output only",
			input: VideoBillingInput{
				Width: 1280, Height: 720, FPS: 24, OutputSeconds: 5, GroupRatio: 1,
			},
			wantTokens:  108000,
			wantRawCost: 216000,
			wantQuota:   int(216000.0 / 1_000_000 * common.QuotaPerUnit),
			wantBasis:   VideoBillingBasisFormula,
		},
		{
			name: "1080p 5s 24fps output only",
			input: VideoBillingInput{
				Width: 1920, Height: 1080, FPS: 24, OutputSeconds: 5, GroupRatio: 1,
			},
			wantTokens:  243000,
			wantRawCost: 486000,
			wantQuota:   int(486000.0 / 1_000_000 * common.QuotaPerUnit),
			wantBasis:   VideoBillingBasisFormula,
		},
		{
			name: "input plus output seconds",
			input: VideoBillingInput{
				Width: 1280, Height: 720, FPS: 24, InputSeconds: 10, OutputSeconds: 5, GroupRatio: 1,
			},
			wantTokens:  324000,
			wantRawCost: 648000,
			wantQuota:   int(648000.0 / 1_000_000 * common.QuotaPerUnit),
			wantBasis:   VideoBillingBasisFormula,
		},
		{
			name: "draft multiplier",
			input: VideoBillingInput{
				Width: 1280, Height: 720, FPS: 24, OutputSeconds: 5, GroupRatio: 1, Draft: true,
			},
			wantTokens:  108000,
			wantRawCost: 108000,
			wantQuota:   int(108000.0 / 1_000_000 * common.QuotaPerUnit),
			wantBasis:   VideoBillingBasisFormula,
		},
		{
			name: "conservative pre-charge",
			input: VideoBillingInput{
				Width: 1280, Height: 720, FPS: 24, OutputSeconds: 5, GroupRatio: 1,
			},
			preCharge:   true,
			wantTokens:  108000,
			wantRawCost: 270000,
			wantQuota:   int(270000.0 / 1_000_000 * common.QuotaPerUnit),
			wantBasis:   VideoBillingBasisFormula,
		},
		{
			name: "group ratio scales quota",
			input: VideoBillingInput{
				Width: 1280, Height: 720, FPS: 24, OutputSeconds: 5, GroupRatio: 1.5,
			},
			wantTokens:  108000,
			wantRawCost: 216000,
			wantQuota:   int(216000.0 / 1_000_000 * common.QuotaPerUnit * 1.5),
			wantBasis:   VideoBillingBasisFormula,
		},
		{
			name: "upstream usage total tokens overrides formula",
			input: VideoBillingInput{
				Width: 1920, Height: 1080, FPS: 24, OutputSeconds: 5, GroupRatio: 1, UpstreamTotalTokens: 1000,
			},
			wantTokens:  1000,
			wantRawCost: 2000,
			wantQuota:   int(2000.0 / 1_000_000 * common.QuotaPerUnit),
			wantBasis:   VideoBillingBasisUpstreamUsage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateVideoBilling(profile, tt.input, tt.preCharge)
			require.Equal(t, tt.wantTokens, got.Tokens)
			require.Equal(t, tt.wantRawCost, got.RawCost)
			require.Equal(t, tt.wantQuota, got.Quota)
			require.Equal(t, tt.wantBasis, got.Basis)
		})
	}
}

func TestVideoBillingCalculatorUsesFallbacks(t *testing.T) {
	resetProfitSettingForVideoBillingTest(t)
	profile := videobilling.VideoBillingProfile{
		Mode:                    videobilling.ModeFormula,
		UnitPrice:               1,
		FallbackFPS:             24,
		FallbackWidth:           1280,
		FallbackHeight:          720,
		FallbackDurationSeconds: 5,
		ConservativeMultiplier:  1,
	}

	got := CalculateVideoBilling(profile, VideoBillingInput{GroupRatio: 1}, false)
	require.Equal(t, 108000, got.Tokens)
}

func TestVideoBillingUnitPriceResolution(t *testing.T) {
	resetProfitSettingForVideoBillingTest(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.default_markup_percent":           "30",
		"profit_setting.upstream_cost_per_million_tokens": "1",
		"profit_setting.apply_to_default_video_profiles":  "true",
	}))

	base := videobilling.VideoBillingProfile{
		Mode:                    videobilling.ModeFormula,
		FallbackFPS:             24,
		FallbackWidth:           1280,
		FallbackHeight:          720,
		FallbackDurationSeconds: 5,
		ConservativeMultiplier:  1.25,
	}

	explicit := base
	explicit.UnitPrice = 9
	gotExplicit := CalculateVideoBilling(explicit, VideoBillingInput{GroupRatio: 1}, false)
	require.Equal(t, 9.0, gotExplicit.RetailUnitPrice)
	require.Equal(t, 1.0, gotExplicit.UpstreamUnitCost)
	require.Equal(t, 9.0*108000, gotExplicit.RawCost)

	derived := base
	derived.UnitPrice = 0
	gotDerived := CalculateVideoBilling(derived, VideoBillingInput{GroupRatio: 1}, false)
	require.InDelta(t, 1.3, gotDerived.RetailUnitPrice, 0.000001)
	require.Equal(t, 1.0, gotDerived.UpstreamUnitCost)
	require.Equal(t, 30.0, gotDerived.MarkupPercent)
	require.InDelta(t, 140400.0, gotDerived.RawCost, 0.000001)

	resetProfitSettingForVideoBillingTest(t)
	// With no per-profile UnitPrice and no profit_setting upstream cost,
	// the resolver must report a zero price rather than silently fall back
	// to a built-in magic number — the OSS binary ships no defaults.
	gotLegacy := CalculateVideoBilling(derived, VideoBillingInput{GroupRatio: 1}, false)
	require.Zero(t, gotLegacy.RetailUnitPrice)
	require.Zero(t, gotLegacy.UpstreamUnitCost)
	require.Zero(t, gotLegacy.RawCost)
	require.Zero(t, gotLegacy.Quota)
}

func TestVideoBillingResolutionAndWithVideoOverride(t *testing.T) {
	resetProfitSettingForVideoBillingTest(t)
	profile := videobilling.VideoBillingProfile{
		Mode:                    videobilling.ModeFormula,
		UnitPrice:               6.39,
		UnitPriceWithVideo:      3.89,
		FallbackFPS:             24,
		FallbackWidth:           1280,
		FallbackHeight:          720,
		FallbackDurationSeconds: 5,
		ConservativeMultiplier:  1,
		MinTokensWithVideo:      150_000,
		UnitPriceByResolution: map[string]float64{
			"720p":  6.39,
			"1080p": 7.08,
		},
		UnitPriceWithVideoByResolution: map[string]float64{
			"720p":  3.89,
			"1080p": 4.31,
		},
	}

	gotPlain720 := CalculateVideoBilling(profile, VideoBillingInput{
		Width: 1280, Height: 720, FPS: 24, OutputSeconds: 5, GroupRatio: 1, Resolution: "720p",
	}, false)
	require.Equal(t, 6.39, gotPlain720.RetailUnitPrice)

	gotPlain1080 := CalculateVideoBilling(profile, VideoBillingInput{
		Width: 1920, Height: 1080, FPS: 24, OutputSeconds: 5, GroupRatio: 1, Resolution: "1080p",
	}, false)
	require.Equal(t, 7.08, gotPlain1080.RetailUnitPrice)

	gotWithVideo720 := CalculateVideoBilling(profile, VideoBillingInput{
		Width: 1280, Height: 720, FPS: 24, OutputSeconds: 5, GroupRatio: 1, Resolution: "720p", HasReferenceMedia: true,
	}, false)
	require.Equal(t, 3.89, gotWithVideo720.RetailUnitPrice)

	gotWithVideo1080 := CalculateVideoBilling(profile, VideoBillingInput{
		Width: 1920, Height: 1080, FPS: 24, OutputSeconds: 5, GroupRatio: 1, Resolution: "1080p", HasReferenceMedia: true,
	}, false)
	require.Equal(t, 4.31, gotWithVideo1080.RetailUnitPrice)

	// Resolution alias unknown to the table → fall back to UnitPriceWithVideo
	gotWithVideoUnknown := CalculateVideoBilling(profile, VideoBillingInput{
		Width: 832, Height: 480, FPS: 24, OutputSeconds: 5, GroupRatio: 1, Resolution: "480p", HasReferenceMedia: true,
	}, false)
	require.Equal(t, 3.89, gotWithVideoUnknown.RetailUnitPrice)
}

func TestVideoBillingMinTokensWithVideoFloor(t *testing.T) {
	resetProfitSettingForVideoBillingTest(t)
	profile := videobilling.VideoBillingProfile{
		Mode:                    videobilling.ModeFormula,
		UnitPrice:               1,
		FallbackFPS:             24,
		FallbackWidth:           1280,
		FallbackHeight:          720,
		FallbackDurationSeconds: 5,
		UseUpstreamUsage:        true,
		ConservativeMultiplier:  1,
		MinTokensWithVideo:      200_000,
	}

	// formula tokens (5 × 1280 × 720 × 24 / 1024 = 108_000) < 200_000 → floor
	belowFloor := CalculateVideoBilling(profile, VideoBillingInput{
		OutputSeconds: 5, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1, HasReferenceMedia: true, InputSeconds: 0,
	}, false)
	require.Equal(t, 200_000, belowFloor.Tokens)

	// upstream-reported value already above floor → keep as-is
	aboveFloor := CalculateVideoBilling(profile, VideoBillingInput{
		OutputSeconds: 5, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1, HasReferenceMedia: true,
		UpstreamTotalTokens: 500_000,
	}, false)
	require.Equal(t, 500_000, aboveFloor.Tokens)

	// floor only applies when reference media is present
	noReference := CalculateVideoBilling(profile, VideoBillingInput{
		OutputSeconds: 5, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1,
	}, false)
	require.Equal(t, 108_000, noReference.Tokens)
}

func TestVideoBillingReferencePrechargeUsesHigherMultiplier(t *testing.T) {
	resetProfitSettingForVideoBillingTest(t)
	profile := videobilling.VideoBillingProfile{
		Mode:                            videobilling.ModeFormula,
		UnitPrice:                       1,
		FallbackFPS:                     24,
		FallbackWidth:                   1280,
		FallbackHeight:                  720,
		FallbackDurationSeconds:         5,
		UseUpstreamUsage:                true,
		ConservativeMultiplier:          1.25,
		ReferenceConservativeMultiplier: 2,
	}

	plain720 := CalculateVideoBilling(profile, VideoBillingInput{
		OutputSeconds: 5, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1,
	}, true)
	require.Equal(t, 67500, plain720.Quota)

	plain1080 := CalculateVideoBilling(profile, VideoBillingInput{
		OutputSeconds: 5, Width: 1920, Height: 1080, FPS: 24, GroupRatio: 1,
	}, true)
	require.Equal(t, 151875, plain1080.Quota)

	referencePrecharge := CalculateVideoBilling(profile, VideoBillingInput{
		InputSeconds: 5, OutputSeconds: 5, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1, HasReferenceMedia: true,
	}, true)
	require.Equal(t, 216000, referencePrecharge.Quota)
	require.GreaterOrEqual(t, referencePrecharge.Quota, 162450)

	extendPrecharge := CalculateVideoBilling(profile, VideoBillingInput{
		InputSeconds: 8, OutputSeconds: 8, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1, HasReferenceMedia: true,
	}, true)
	require.Equal(t, 345600, extendPrecharge.Quota)
	require.GreaterOrEqual(t, extendPrecharge.Quota, 194850)
}

func resetProfitSettingForVideoBillingTest(t *testing.T) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.default_markup_percent":           "0",
		"profit_setting.upstream_cost_per_million_tokens": "0",
		"profit_setting.apply_to_default_video_profiles":  "true",
	}))
}
