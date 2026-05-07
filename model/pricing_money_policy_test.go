package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPricingMoneyPolicyTest(t *testing.T) {
	t.Helper()
	initCol()
	ensureModelTestSchema(t, &Channel{}, &Ability{}, &RetailPricingPolicy{}, &Model{}, &Vendor{})
	for _, table := range []string{"retail_pricing_policies", "abilities", "channels", "models", "vendors"} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	InvalidatePricingCache()
	t.Cleanup(func() {
		for _, table := range []string{"retail_pricing_policies", "abilities", "channels", "models", "vendors"} {
			DB.Exec("DELETE FROM " + table)
		}
		InvalidatePricingCache()
	})
}

func TestPricingUsesMoneyPolicyAnchorForVideoModel(t *testing.T) {
	setupPricingMoneyPolicyTest(t)
	const modelName = "zz-money-video-model"
	require.NoError(t, DB.Create(&Channel{
		Id:     9201,
		Type:   constant.ChannelTypeSora,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "money-video-channel",
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     DefaultPricingGroup,
		Model:     modelName,
		ChannelId: 9201,
		Enabled:   true,
	}).Error)
	require.NoError(t, DB.Create(&RetailPricingPolicy{
		PublicModel:  modelName,
		Group:        DefaultPricingGroup,
		EndpointType: string(constant.EndpointTypeOpenAIVideo),
		PricingMode:  PricingModeFixedRule,
		Currency:     "USD",
		BillingRuleJSON: `{
			"schema_version": 1,
			"profile_type": "video_rule_matrix",
			"currency": "USD",
			"rules": [{
				"id": "base",
				"rates": {
					"720p|5": {"price": {"amount_micros": 1000000, "basis": "per_unit", "currency": "USD"}},
					"480p|5": {"price": {"amount_micros": 500000, "basis": "per_unit", "currency": "USD"}}
				}
			}]
		}`,
	}).Error)

	pricingByName := make(map[string]Pricing)
	for _, pricing := range GetPricing() {
		pricingByName[pricing.ModelName] = pricing
	}
	pricing, ok := pricingByName[modelName]
	require.True(t, ok)

	assert.Equal(t, int64(500000), pricing.MoneyPricingAmountMicros)
	assert.Equal(t, "USD", pricing.MoneyPricingCurrency)
	assert.Equal(t, "video", pricing.MoneyPricingUnit)
	assert.Equal(t, PricingModeFixedRule, pricing.MoneyPricingMode)
	assert.Equal(t, 1, pricing.QuotaType)
	assert.Equal(t, 0.5, pricing.ModelPrice)
}
