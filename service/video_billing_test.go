package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
