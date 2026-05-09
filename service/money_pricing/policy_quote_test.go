package money_pricing

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelModelCost{}, &model.RetailPricingPolicy{}, &model.FxRate{}); err != nil {
		panic("failed to migrate: " + err.Error())
	}
	os.Exit(m.Run())
}

func setupPolicyQuoteTest(t *testing.T) {
	t.Helper()
	for _, table := range []string{"fx_rates", "channel_model_costs", "abilities", "channels", "retail_pricing_policies"} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
	t.Cleanup(func() {
		for _, table := range []string{"fx_rates", "channel_model_costs", "abilities", "channels", "retail_pricing_policies"} {
			model.DB.Exec("DELETE FROM " + table)
		}
	})
}

func moneyUsageProfileJSON(amountMicros int64) string {
	return fmt.Sprintf(`{"schema_version":1,"profile_type":"money_usage_pricing","currency":"USD","rates":{"input_token":{"amount_micros":%d,"basis":"per_million_units","currency":"USD"}}}`, amountMicros)
}

func TestQuoteRetailPricingPolicyCostPlusUsesPolicyFallbackProfile(t *testing.T) {
	setupPolicyQuoteTest(t)
	policy := &model.RetailPricingPolicy{
		PublicModel:     "public-chat",
		Group:           model.DefaultPricingGroup,
		EndpointType:    "openai",
		PricingMode:     model.PricingModeCostPlus,
		BillingRuleJSON: moneyUsageProfileJSON(2_000_000),
		MarkupBps:       3000,
		Currency:        "USD",
	}

	quote, err := QuoteRetailPricingPolicy(policy, MoneyUsageFeatures{
		InputTokens: 1_000_000,
	}, "USD")

	require.NoError(t, err)
	require.Equal(t, int64(2_000_000), quote.UpstreamCostMicros)
	require.Equal(t, int64(2_600_000), quote.RetailAmountMicros)
	require.Equal(t, "public-chat", quote.PublicModel)
}

func TestQuoteRetailPricingPolicyCostPlusUsesSelectedChannelCost(t *testing.T) {
	setupPolicyQuoteTest(t)
	require.NoError(t, model.DB.Create(&model.ChannelModelCost{
		ChannelId:       7,
		UpstreamModel:   "upstream-chat",
		EndpointType:    "openai",
		BillingRuleJSON: moneyUsageProfileJSON(1_000_000),
		Source:          "manual",
		Enabled:         true,
	}).Error)
	policy := &model.RetailPricingPolicy{
		PublicModel:  "public-chat",
		Group:        model.DefaultPricingGroup,
		EndpointType: "openai",
		PricingMode:  model.PricingModeCostPlus,
		MarkupBps:    3000,
		Currency:     "USD",
	}

	quote, err := QuoteRetailPricingPolicy(policy, MoneyUsageFeatures{
		ChannelID:     7,
		UpstreamModel: "upstream-chat",
		InputTokens:   1_000_000,
	}, "USD")

	require.NoError(t, err)
	require.Equal(t, 7, quote.ChannelID)
	require.Equal(t, "upstream-chat", quote.UpstreamModel)
	require.Equal(t, int64(1_000_000), quote.UpstreamCostMicros)
	require.Equal(t, int64(1_300_000), quote.RetailAmountMicros)
}

func TestQuoteRetailPricingPolicyCostPlusUsesConservativeChannelCost(t *testing.T) {
	setupPolicyQuoteTest(t)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 11, Key: "k1", Name: "c1", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 12, Key: "k2", Name: "c2", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: model.DefaultPricingGroup, Model: "public-chat", ChannelId: 11, Enabled: true}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: model.DefaultPricingGroup, Model: "public-chat", ChannelId: 12, Enabled: true}).Error)
	require.NoError(t, model.DB.Create(&model.ChannelModelCost{
		ChannelId:       11,
		UpstreamModel:   "public-chat",
		EndpointType:    "openai",
		BillingRuleJSON: moneyUsageProfileJSON(1_000_000),
		Source:          "manual",
		Enabled:         true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ChannelModelCost{
		ChannelId:       12,
		UpstreamModel:   "public-chat",
		EndpointType:    "openai",
		BillingRuleJSON: moneyUsageProfileJSON(1_750_000),
		Source:          "manual",
		Enabled:         true,
	}).Error)
	policy := &model.RetailPricingPolicy{
		PublicModel:  "public-chat",
		Group:        model.DefaultPricingGroup,
		EndpointType: "openai",
		PricingMode:  model.PricingModeCostPlus,
		MarkupBps:    2000,
		Currency:     "USD",
	}

	quote, err := QuoteRetailPricingPolicy(policy, MoneyUsageFeatures{
		InputTokens: 1_000_000,
	}, "USD")

	require.NoError(t, err)
	require.Equal(t, 12, quote.ChannelID)
	require.Equal(t, int64(1_750_000), quote.UpstreamCostMicros)
	require.Equal(t, int64(2_100_000), quote.RetailAmountMicros)
}

func TestQuoteRetailPricingPolicyCostPlusEmptyWithoutCostReturnsNotFound(t *testing.T) {
	setupPolicyQuoteTest(t)
	policy := &model.RetailPricingPolicy{
		PublicModel:  "public-chat",
		Group:        model.DefaultPricingGroup,
		EndpointType: "openai",
		PricingMode:  model.PricingModeCostPlus,
		Currency:     "USD",
	}

	_, err := QuoteRetailPricingPolicy(policy, MoneyUsageFeatures{
		InputTokens: 1_000_000,
	}, "USD")

	require.Error(t, err)
	require.True(t, errors.Is(err, model.ErrMoneyPricingPolicyNotFound))
}
