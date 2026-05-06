package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
	"github.com/stretchr/testify/require"
)

func TestTaskBillingBreakdownSanitizesByVisibilityMode(t *testing.T) {
	task := fixtureBillingTask()

	credits := BuildTaskBillingBreakdown(task, BillingVisibilityCredits)
	require.Equal(t, BillingVisibilityCredits, credits["mode"])
	require.Equal(t, "Credits", credits["label"])
	require.Equal(t, 10.89, credits["charged_credits"])
	require.NotContains(t, credits, "charged_quota")
	require.NotContains(t, credits, "unit_price_per_million")
	require.NotContains(t, credits, "upstream_unit_cost_per_million")
	require.NotContains(t, credits, "gross_margin_quota")

	detailed := BuildTaskBillingBreakdown(task, BillingVisibilityDetailed)
	require.Equal(t, BillingVisibilityDetailed, detailed["mode"])
	require.Equal(t, 10.89, detailed["charged_credits"])
	require.Equal(t, 54450, detailed["charged_quota"])
	require.Equal(t, "doubao-seedance-2-0-fast-260128", detailed["model"])
	require.Equal(t, VideoBillingBasisUpstreamUsage, detailed["basis"])
	require.Equal(t, 108900, detailed["tokens"])
	require.Equal(t, 1.0, detailed["unit_price_per_million"])
	require.Equal(t, 1.0, detailed["group_ratio"])
	require.Equal(t, 67500, detailed["precharged_quota"])
	require.Equal(t, 54450, detailed["actual_quota"])
	require.Equal(t, -13050, detailed["adjustment_quota"])
	require.Equal(t, "video_formula:v1:testhash", detailed["pricing_version"])
	require.NotContains(t, detailed, "upstream_unit_cost_per_million")
	require.NotContains(t, detailed, "gross_margin_quota")

	internal := BuildTaskBillingBreakdown(task, BillingVisibilityInternal)
	require.Equal(t, BillingVisibilityInternal, internal["mode"])
	require.Equal(t, 0.8, internal["upstream_unit_cost_per_million"])
	require.Equal(t, 43560, internal["upstream_cost_quota"])
	require.Equal(t, 10890, internal["gross_margin_quota"])
	require.Equal(t, 20.0, internal["gross_margin_percent"])
}

func TestLogBillingBreakdownSanitizesByVisibilityMode(t *testing.T) {
	other := map[string]interface{}{
		"is_task":            true,
		"billing_mode":       videobilling.ModeFormula,
		"billing_profile":    "doubao-seedance-2-0-fast-260128",
		"billing_basis":      VideoBillingBasisUpstreamUsage,
		"estimated_tokens":   108900,
		"estimated_quota":    67500,
		"actual_quota":       54450,
		"pre_consumed_quota": 67500,
		"retail_unit_price":  1.0,
		"upstream_unit_cost": 0.8,
		"markup_percent":     25.0,
		"pricing_version":    "video_formula:v1:testhash",
		"pricing_hash":       "testhash",
		"group_ratio":        1.0,
		"video_params": map[string]any{
			"output_seconds": 5,
			"width":          1280,
			"height":         720,
			"fps":            24,
		},
	}
	log := &model.Log{
		Type:      model.LogTypeRefund,
		Quota:     13050,
		ModelName: "doubao-seedance-2-0-fast-260128",
		Other:     common.MapToJsonStr(other),
	}

	detailed := BuildLogBillingBreakdown(log, BillingVisibilityDetailed)
	require.Equal(t, BillingVisibilityDetailed, detailed["mode"])
	require.Equal(t, 10.89, detailed["charged_credits"])
	require.Equal(t, 54450, detailed["actual_quota"])
	require.Equal(t, -13050, detailed["adjustment_quota"])
	require.Equal(t, "video_formula:v1:testhash", detailed["pricing_version"])
	require.NotContains(t, detailed, "upstream_unit_cost_per_million")
	require.NotContains(t, detailed, "gross_margin_quota")

	internal := BuildLogBillingBreakdown(log, BillingVisibilityInternal)
	require.Equal(t, 43560, internal["upstream_cost_quota"])
	require.Equal(t, 10890, internal["gross_margin_quota"])
}

func TestSanitizeLogOtherForBillingVisibilityRemovesVideoAliasFields(t *testing.T) {
	other := map[string]interface{}{
		"billing_mode":             videobilling.ModeFormula,
		"estimated_tokens":         108900,
		"video_estimated_tokens":   108900,
		"video_retail_unit_price":  1.0,
		"video_upstream_unit_cost": 0.8,
		"video_markup_percent":     25.0,
		"upstream_unit_cost":       0.8,
		"markup_percent":           25.0,
	}
	log := &model.Log{Other: common.MapToJsonStr(other)}

	SanitizeLogOtherForBillingVisibility(log, BillingVisibilityDetailed)

	sanitized, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	require.NotContains(t, sanitized, "video_estimated_tokens")
	require.NotContains(t, sanitized, "video_retail_unit_price")
	require.NotContains(t, sanitized, "video_upstream_unit_cost")
	require.NotContains(t, sanitized, "video_markup_percent")
	require.NotContains(t, sanitized, "upstream_unit_cost")
	require.NotContains(t, sanitized, "markup_percent")
	require.Contains(t, sanitized, "estimated_tokens")
}

func fixtureBillingTask() *model.Task {
	return &model.Task{
		TaskID: "task_fixture_billing",
		Quota:  54450,
		Group:  "default",
		Properties: model.Properties{
			OriginModelName: "doubao-seedance-2-0-fast-260128",
		},
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				BillingMode:      videobilling.ModeFormula,
				BillingProfile:   "doubao-seedance-2-0-fast-260128",
				BillingBasis:     VideoBillingBasisUpstreamUsage,
				OriginModelName:  "doubao-seedance-2-0-fast-260128",
				GroupRatio:       1,
				EstimatedTokens:  108900,
				EstimatedQuota:   67500,
				RetailUnitPrice:  1,
				UpstreamUnitCost: 0.8,
				MarkupPercent:    25,
				PricingVersion:   "video_formula:v1:testhash",
				PricingHash:      "testhash",
				VideoParams: map[string]any{
					"output_seconds": 5,
					"width":          1280,
					"height":         720,
					"fps":            24,
				},
			},
		},
	}
}
