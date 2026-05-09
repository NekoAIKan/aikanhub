package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validMoneyUsageProfileJSON = `{"schema_version":1,"profile_type":"money_usage_pricing","rates":{"request":{"amount_micros":1000,"basis":"per_unit","currency":"USD"}}}`

func setupMoneyPricingPolicyTest(t *testing.T) {
	t.Helper()
	ensureModelTestSchema(t, &Channel{}, &Ability{}, &ChannelModelCost{}, &RetailPricingPolicy{})
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM channel_model_costs").Error)
	require.NoError(t, DB.Exec("DELETE FROM retail_pricing_policies").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM abilities")
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM channel_model_costs")
		DB.Exec("DELETE FROM retail_pricing_policies")
	})
}

func TestChannelModelCostLookupIgnoresDisabledRows(t *testing.T) {
	setupMoneyPricingPolicyTest(t)
	require.NoError(t, DB.Create(&ChannelModelCost{
		ChannelId:       7,
		UpstreamModel:   "upstream-model",
		EndpointType:    "chat",
		BillingRuleJSON: validMoneyUsageProfileJSON,
		Source:          "manual",
		Enabled:         false,
	}).Error)
	require.NoError(t, DB.Create(&ChannelModelCost{
		ChannelId:       7,
		UpstreamModel:   "upstream-model",
		EndpointType:    "chat",
		BillingRuleJSON: validMoneyUsageProfileJSON,
		Source:          "manual",
		Enabled:         true,
	}).Error)

	cost, err := GetEnabledChannelModelCost(7, "upstream-model", "chat")
	require.NoError(t, err)
	assert.True(t, cost.Enabled)
	assert.Equal(t, "manual", cost.Source)
}

func TestListEnabledChannelModelCostsForPublicModelHonorsMapping(t *testing.T) {
	setupMoneyPricingPolicyTest(t)
	mapping := `{"public-model":"upstream-model"}`
	require.NoError(t, DB.Create(&Channel{
		Id:           71,
		Key:          "test-key",
		Name:         "mapped-channel",
		Status:       common.ChannelStatusEnabled,
		ModelMapping: &mapping,
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     DefaultPricingGroup,
		Model:     "public-model",
		ChannelId: 71,
		Enabled:   true,
	}).Error)
	require.NoError(t, DB.Create(&ChannelModelCost{
		ChannelId:       71,
		UpstreamModel:   "upstream-model",
		EndpointType:    "chat",
		BillingRuleJSON: validMoneyUsageProfileJSON,
		Source:          "manual",
		Enabled:         true,
	}).Error)

	costs, err := ListEnabledChannelModelCostsForPublicModel("public-model", DefaultPricingGroup, "chat")
	require.NoError(t, err)
	require.Len(t, costs, 1)
	assert.Equal(t, "upstream-model", costs[0].UpstreamModel)
}

func TestChannelModelCostRejectsInvalidProfileJSON(t *testing.T) {
	setupMoneyPricingPolicyTest(t)

	err := DB.Create(&ChannelModelCost{
		ChannelId:       7,
		UpstreamModel:   "upstream-model",
		EndpointType:    "chat",
		BillingRuleJSON: `{"schema_version":`,
		Source:          "manual",
		Enabled:         true,
	}).Error
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMoneyPricingProfileInvalid))
}

func TestChannelModelCostRejectsForbiddenCreditFields(t *testing.T) {
	setupMoneyPricingPolicyTest(t)

	err := DB.Create(&ChannelModelCost{
		ChannelId:       7,
		UpstreamModel:   "upstream-model",
		EndpointType:    "chat",
		BillingRuleJSON: `{"schema_version":1,"profile_type":"money_usage_pricing","credit_unit":{"per_usd":200},"rates":{"request":{"amount_micros":1000,"basis":"per_unit","currency":"USD"}}}`,
		Source:          "manual",
		Enabled:         true,
	}).Error
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMoneyPricingProfileInvalid))
}

func TestChannelModelCostRejectsMalformedMoneyUsageProfile(t *testing.T) {
	setupMoneyPricingPolicyTest(t)

	testCases := []struct {
		name string
		raw  string
	}{
		{
			name: "fractional schema version",
			raw:  `{"schema_version":1.5,"profile_type":"money_usage_pricing","rates":{"request":{"amount_micros":1000,"basis":"per_unit","currency":"USD"}}}`,
		},
		{
			name: "unsupported unit",
			raw:  `{"schema_version":1,"profile_type":"money_usage_pricing","rates":{"provider_credit":{"amount_micros":1000,"basis":"per_unit","currency":"USD"}}}`,
		},
		{
			name: "negative amount",
			raw:  `{"schema_version":1,"profile_type":"money_usage_pricing","rates":{"request":{"amount_micros":-1,"basis":"per_unit","currency":"USD"}}}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := DB.Create(&ChannelModelCost{
				ChannelId:       7,
				UpstreamModel:   "upstream-model",
				EndpointType:    "chat",
				BillingRuleJSON: tc.raw,
				Source:          "manual",
				Enabled:         true,
			}).Error
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrMoneyPricingProfileInvalid))
		})
	}
}

func TestRetailPricingPolicyFallsBackToDefaultGroup(t *testing.T) {
	setupMoneyPricingPolicyTest(t)
	require.NoError(t, DB.Create(&RetailPricingPolicy{
		PublicModel:  "public-model",
		Group:        DefaultPricingGroup,
		EndpointType: "chat",
		PricingMode:  PricingModeCostPlus,
		MarkupBps:    1200,
		Currency:     "usd",
		Enabled:      true,
	}).Error)

	policy, err := GetEnabledRetailPricingPolicy("public-model", "enterprise", "chat")
	require.NoError(t, err)
	assert.Equal(t, DefaultPricingGroup, policy.Group)
	assert.Equal(t, int64(1200), policy.MarkupBps)
	assert.Equal(t, "USD", policy.Currency)
}

func TestRetailPricingPolicyExactGroupWins(t *testing.T) {
	setupMoneyPricingPolicyTest(t)
	require.NoError(t, DB.Create(&RetailPricingPolicy{
		PublicModel:  "public-model",
		Group:        DefaultPricingGroup,
		EndpointType: "chat",
		PricingMode:  PricingModeCostPlus,
		MarkupBps:    100,
		Currency:     "USD",
		Enabled:      true,
	}).Error)
	require.NoError(t, DB.Create(&RetailPricingPolicy{
		PublicModel:  "public-model",
		Group:        "enterprise",
		EndpointType: "chat",
		PricingMode:  PricingModeCostPlus,
		MarkupBps:    200,
		Currency:     "USD",
		Enabled:      true,
	}).Error)

	policy, err := GetEnabledRetailPricingPolicy("public-model", "enterprise", "chat")
	require.NoError(t, err)
	assert.Equal(t, "enterprise", policy.Group)
	assert.Equal(t, int64(200), policy.MarkupBps)
}

func TestRetailPricingPolicyLookupFallsBackToDefaultGroup(t *testing.T) {
	policies := []RetailPricingPolicy{
		{
			Id:           1,
			PublicModel:  "public-model",
			Group:        DefaultPricingGroup,
			EndpointType: "chat",
			PricingMode:  PricingModeCostPlus,
			MarkupBps:    100,
			Currency:     "USD",
		},
		{
			Id:           2,
			PublicModel:  "public-model",
			Group:        "enterprise",
			EndpointType: "chat",
			PricingMode:  PricingModeCostPlus,
			MarkupBps:    200,
			Currency:     "USD",
		},
	}
	lookup := BuildRetailPricingPolicyLookup(policies)

	exact, ok := lookup.Lookup("public-model", "enterprise", "chat")
	require.True(t, ok)
	assert.Equal(t, int64(200), exact.MarkupBps)

	fallback, ok := lookup.Lookup("public-model", "trial", "chat")
	require.True(t, ok)
	assert.Equal(t, DefaultPricingGroup, fallback.Group)
	assert.Equal(t, int64(100), fallback.MarkupBps)
}

func TestRetailPricingPolicyRequiresFixedRuleProfile(t *testing.T) {
	setupMoneyPricingPolicyTest(t)

	err := DB.Create(&RetailPricingPolicy{
		PublicModel:  "public-model",
		Group:        DefaultPricingGroup,
		EndpointType: "chat",
		PricingMode:  PricingModeFixedRule,
		Currency:     "USD",
		Enabled:      true,
	}).Error
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMoneyPricingProfileInvalid))
}

func TestRetailPricingPolicyCostPlusAllowsEmptyProfile(t *testing.T) {
	setupMoneyPricingPolicyTest(t)

	require.NoError(t, DB.Create(&RetailPricingPolicy{
		PublicModel:  "public-model",
		Group:        DefaultPricingGroup,
		EndpointType: "chat",
		PricingMode:  PricingModeCostPlus,
		Currency:     "USD",
		Enabled:      true,
	}).Error)
}
