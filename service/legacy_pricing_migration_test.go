package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestLegacyPricingMigrationModelPriceBuildsRequestProfile(t *testing.T) {
	profileJSON, err := LegacyModelPriceToMoneyProfileJSON("dall-e-3", 0.04)
	require.NoError(t, err)

	var profile LegacyMoneyUsagePricingProfile
	require.NoError(t, common.Unmarshal([]byte(profileJSON), &profile))

	require.Equal(t, 1, profile.SchemaVersion)
	require.Equal(t, "money_usage_pricing", profile.ProfileType)
	require.Equal(t, "request", profile.EndpointType)
	require.Equal(t, "dall-e-3", profile.Model)
	require.Equal(t, "USD", profile.Currency)

	requestRate := profile.Rates["request"]
	require.Equal(t, int64(40_000), requestRate.AmountMicros)
	require.Equal(t, "per_unit", requestRate.Basis)
	require.Equal(t, "USD", requestRate.Currency)
	require.Equal(t, "legacy_pricing_migration", profile.SourceNote.Source)
	require.Equal(t, "model_price", profile.SourceNote.LegacyKind)
	require.Equal(t, 0.04, profile.SourceNote.ModelPrice)
}

func TestLegacyPricingMigrationModelRatioUsesQuotaPerUnitAndCompletionRatio(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 250_000
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	profileJSON, err := LegacyModelRatioToMoneyProfileJSON("gpt-test", 0.5, 3)
	require.NoError(t, err)

	var profile LegacyMoneyUsagePricingProfile
	require.NoError(t, common.Unmarshal([]byte(profileJSON), &profile))

	require.Equal(t, "chat", profile.EndpointType)
	require.Equal(t, "gpt-test", profile.Model)

	inputRate := profile.Rates["input_token"]
	require.Equal(t, int64(2_000_000), inputRate.AmountMicros)
	require.Equal(t, "per_million_units", inputRate.Basis)
	require.Equal(t, "USD", inputRate.Currency)

	outputRate := profile.Rates["output_token"]
	require.Equal(t, int64(6_000_000), outputRate.AmountMicros)
	require.Equal(t, "per_million_units", outputRate.Basis)
	require.Equal(t, "USD", outputRate.Currency)

	require.Equal(t, "model_ratio", profile.SourceNote.LegacyKind)
	require.Equal(t, 0.5, profile.SourceNote.ModelRatio)
	require.Equal(t, 3.0, profile.SourceNote.CompletionRatio)
	require.Equal(t, 250_000.0, profile.SourceNote.QuotaPerUnit)
}

func TestLegacyPricingMigrationGroupRatioBuildsPolicyDrafts(t *testing.T) {
	drafts, err := LegacyGroupRatioToRetailPolicyDrafts(map[string]float64{
		"default":    1,
		"enterprise": 1.25,
		"vip":        0.8,
	})
	require.NoError(t, err)
	require.Len(t, drafts, 3)

	defaultDraft := requireGroupPolicyDraft(t, drafts, "default")
	require.Equal(t, int64(10_000), defaultDraft.MultiplierBps)
	require.Zero(t, defaultDraft.DiscountBps)
	require.Zero(t, defaultDraft.MarkupBps)

	enterpriseDraft := requireGroupPolicyDraft(t, drafts, "enterprise")
	require.Equal(t, int64(12_500), enterpriseDraft.MultiplierBps)
	require.Zero(t, enterpriseDraft.DiscountBps)
	require.Equal(t, int64(2_500), enterpriseDraft.MarkupBps)

	vipDraft := requireGroupPolicyDraft(t, drafts, "vip")
	require.Equal(t, int64(8_000), vipDraft.MultiplierBps)
	require.Equal(t, int64(2_000), vipDraft.DiscountBps)
	require.Zero(t, vipDraft.MarkupBps)
	require.Equal(t, "retail_multiplier", vipDraft.PolicyType)
	require.Equal(t, "legacy_pricing_migration", vipDraft.SourceNote.Source)
	require.Equal(t, "group_ratio", vipDraft.SourceNote.LegacyKind)

	draftsJSON, err := LegacyGroupRatioRetailPolicyDraftsToJSON(drafts)
	require.NoError(t, err)
	var decoded []LegacyGroupRetailPolicyDraft
	require.NoError(t, common.Unmarshal([]byte(draftsJSON), &decoded))
	require.Equal(t, drafts, decoded)
}

func TestLegacyPricingMigrationRejectsInvalidLegacyValues(t *testing.T) {
	_, err := LegacyModelPriceToMoneyProfileJSON("bad-price", -0.01)
	require.Error(t, err)

	_, err = LegacyModelRatioToMoneyProfileJSON("bad-ratio", -1, 1)
	require.Error(t, err)

	_, err = LegacyModelRatioToMoneyProfileJSON("bad-completion", 1, -1)
	require.Error(t, err)

	_, err = LegacyGroupRatioToRetailPolicyDrafts(map[string]float64{"bad-group": -0.5})
	require.Error(t, err)
}

func requireGroupPolicyDraft(t *testing.T, drafts []LegacyGroupRetailPolicyDraft, group string) LegacyGroupRetailPolicyDraft {
	t.Helper()
	for _, draft := range drafts {
		if draft.Group == group {
			return draft
		}
	}
	t.Fatalf("group policy draft %q not found", group)
	return LegacyGroupRetailPolicyDraft{}
}
