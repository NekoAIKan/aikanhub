package service

import (
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	moneypricing "github.com/QuantumNous/new-api/service/money_pricing"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const moneyRuntimePolicyJSON = `{"schema_version":1,"profile_type":"money_usage_pricing","currency":"USD","rates":{"input_token":{"amount_micros":100,"basis":"per_unit","currency":"USD"},"output_token":{"amount_micros":200,"basis":"per_unit","currency":"USD"},"audio_input_token":{"amount_micros":1000,"basis":"per_unit","currency":"USD"},"audio_output_token":{"amount_micros":2000,"basis":"per_unit","currency":"USD"}}}`

func setupMoneyRuntimeQuotaTest(t *testing.T) {
	t.Helper()
	ensureServiceTestSchema(t, &model.User{}, &model.Token{}, &model.Log{}, &model.MoneyWallet{}, &model.MoneyWalletTransaction{}, &model.Channel{}, &model.Ability{}, &model.ChannelModelCost{}, &model.RetailPricingPolicy{})
	for _, table := range []string{"logs", "money_wallet_transactions", "money_wallets", "channel_model_costs", "abilities", "channels", "retail_pricing_policies", "tokens", "users"} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
	originalQuotaPerUnit := common.QuotaPerUnit
	originalLogConsumeEnabled := common.LogConsumeEnabled
	common.QuotaPerUnit = 500_000
	common.LogConsumeEnabled = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "money",
		"billing_setting.settlement_currency": "USD",
	}))
	t.Cleanup(func() {
		for _, table := range []string{"logs", "money_wallet_transactions", "money_wallets", "channel_model_costs", "abilities", "channels", "retail_pricing_policies", "tokens", "users"} {
			model.DB.Exec("DELETE FROM " + table)
		}
		common.QuotaPerUnit = originalQuotaPerUnit
		common.LogConsumeEnabled = originalLogConsumeEnabled
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
		})
	})
}

func TestMoneyRuntimeSettlementUsesSelectedChannelCostForCostPlus(t *testing.T) {
	setupMoneyRuntimeQuotaTest(t)
	modelName := "money-runtime-cost-plus"
	require.NoError(t, model.DB.Create(&model.RetailPricingPolicy{
		PublicModel:     modelName,
		Group:           model.DefaultPricingGroup,
		EndpointType:    "openai",
		PricingMode:     model.PricingModeCostPlus,
		BillingRuleJSON: `{"schema_version":1,"profile_type":"money_usage_pricing","currency":"USD","rates":{"input_token":{"amount_micros":2000000,"basis":"per_million_units","currency":"USD"}}}`,
		Currency:        "USD",
		Enabled:         true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ChannelModelCost{
		ChannelId:       17,
		UpstreamModel:   "upstream-runtime-cost-plus",
		EndpointType:    "openai",
		BillingRuleJSON: `{"schema_version":1,"profile_type":"money_usage_pricing","currency":"USD","rates":{"input_token":{"amount_micros":1500000,"basis":"per_million_units","currency":"USD"}}}`,
		Source:          "manual",
		Enabled:         true,
	}).Error)
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UsingGroup:      model.DefaultPricingGroup,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         17,
			UpstreamModelName: "upstream-runtime-cost-plus",
		},
		PriceData: types.PriceData{
			MoneyPricingEnabled: true,
			MoneyEndpointType:   "openai",
		},
	}

	_, applied, err := applyRuntimeMoneyUsageQuote(relayInfo, 1, func(activeUnits map[moneypricing.UsageUnit]struct{}, endpointType string) moneypricing.MoneyUsageFeatures {
		require.Contains(t, activeUnits, moneypricing.UsageUnitInputToken)
		return moneypricing.MoneyUsageFeatures{InputTokens: 1_000_000}
	})

	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, relayInfo.PriceData.MoneyActualAmountKnown)
	assert.Equal(t, int64(1_500_000), relayInfo.PriceData.MoneyActualAmountMicros)
	assert.Equal(t, int64(1_500_000), relayInfo.PriceData.MoneyPricingActualUpstreamCostMicros)
}

func TestMoneyRuntimeCostPlusFallsBackToPreconsumeWhenSelectedChannelCostMissing(t *testing.T) {
	setupMoneyRuntimeQuotaTest(t)
	modelName := "money-runtime-cost-plus-missing"
	require.NoError(t, model.DB.Create(&model.RetailPricingPolicy{
		PublicModel:  modelName,
		Group:        model.DefaultPricingGroup,
		EndpointType: "openai",
		PricingMode:  model.PricingModeCostPlus,
		Currency:     "USD",
		Enabled:      true,
	}).Error)
	relayInfo := &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UsingGroup:      model.DefaultPricingGroup,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         18,
			UpstreamModelName: "missing-upstream",
		},
		PriceData: types.PriceData{
			MoneyPricingEnabled:          true,
			MoneyEndpointType:            "openai",
			MoneyPreConsumedAmountKnown:  true,
			MoneyPreConsumedAmountMicros: 123_456,
		},
	}

	quota, applied, err := applyRuntimeMoneyUsageQuote(relayInfo, 1, func(activeUnits map[moneypricing.UsageUnit]struct{}, endpointType string) moneypricing.MoneyUsageFeatures {
		return moneypricing.MoneyUsageFeatures{InputTokens: 1_000_000}
	})

	require.NoError(t, err)
	require.True(t, applied)
	assert.Equal(t, compatibilityQuotaFromMoneyAmount(123_456), quota)
	require.True(t, relayInfo.PriceData.MoneyActualAmountKnown)
	assert.Equal(t, int64(123_456), relayInfo.PriceData.MoneyActualAmountMicros)
}

func seedMoneyRuntimePolicy(t *testing.T, modelName string, profile string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.RetailPricingPolicy{
		PublicModel:     modelName,
		Group:           model.DefaultPricingGroup,
		EndpointType:    "openai",
		PricingMode:     model.PricingModeFixedRule,
		BillingRuleJSON: profile,
		Currency:        "USD",
		Enabled:         true,
	}).Error)
}

func newMoneyRuntimeRelayInfo(userID int, token model.Token, requestID string, modelName string, preConsumedMicros int64) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         token.Id,
		TokenKey:        token.Key,
		RequestId:       requestID,
		OriginModelName: modelName,
		UsingGroup:      model.DefaultPricingGroup,
		UserGroup:       model.DefaultPricingGroup,
		StartTime:       time.Now(),
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
		PriceData: types.PriceData{
			UsePrice:                       true,
			GroupRatioInfo:                 types.GroupRatioInfo{GroupRatio: 1},
			MoneyPricingEnabled:            true,
			MoneyEndpointType:              "openai",
			MoneyPricingMode:               model.PricingModeFixedRule,
			MoneySettlementCurrency:        "USD",
			MoneyPreConsumedAmountMicros:   preConsumedMicros,
			MoneyPreConsumedAmountKnown:    true,
			MoneyPricingUpstreamCostMicros: preConsumedMicros,
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 1,
		},
	}
}

func prepareMoneyRuntimeRequest(t *testing.T, userID int, tokenKey string, requestID string, modelName string, preConsumedMicros int64) (*gin.Context, *relaycommon.RelayInfo, model.Token) {
	t.Helper()
	token := seedMoneyBillingSessionUserAndToken(t, userID, tokenKey)
	ctx := newMoneyBillingSessionContext()
	ctx.Set("token_name", tokenKey)
	ctx.Set("username", tokenKey)
	ctx.Set(common.RequestIdKey, requestID)
	relayInfo := newMoneyRuntimeRelayInfo(userID, token, requestID, modelName, preConsumedMicros)
	apiErr := PreConsumeBilling(ctx, compatibilityQuotaFromMoneyAmount(preConsumedMicros), relayInfo)
	require.Nil(t, apiErr)
	return ctx, relayInfo, token
}

func TestPostAudioConsumeQuotaUsesMoneyUsageExactAmount(t *testing.T) {
	setupMoneyRuntimeQuotaTest(t)
	modelName := "money-runtime-audio"
	seedMoneyRuntimePolicy(t, modelName, moneyRuntimePolicyJSON)
	ctx, relayInfo, token := prepareMoneyRuntimeRequest(t, 9101, "money-audio-token", "req-money-audio", modelName, 20_000)

	PostAudioConsumeQuota(ctx, relayInfo, &dto.Usage{
		PromptTokens:     13,
		CompletionTokens: 2,
		TotalTokens:      15,
		PromptTokensDetails: dto.InputTokenDetails{
			TextTokens:  10,
			AudioTokens: 3,
		},
		CompletionTokenDetails: dto.OutputTokenDetails{
			AudioTokens: 2,
		},
	}, "")

	require.True(t, relayInfo.PriceData.MoneyActualAmountKnown)
	assert.Equal(t, int64(8_000), relayInfo.PriceData.MoneyActualAmountMicros)
	wallet := fetchMoneyBillingSessionWallet(t, 9101)
	assert.Equal(t, int64(1_992_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	settledToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_992_000), settledToken.RemainAmountMicros)
	assert.Equal(t, int64(8_000), settledToken.UsedAmountMicros)
}

func TestPostWssConsumeQuotaUsesMoneyUsageExactAmount(t *testing.T) {
	setupMoneyRuntimeQuotaTest(t)
	modelName := "money-runtime-realtime"
	seedMoneyRuntimePolicy(t, modelName, moneyRuntimePolicyJSON)
	ctx, relayInfo, token := prepareMoneyRuntimeRequest(t, 9102, "money-wss-token", "req-money-wss", modelName, 30_000)

	PostWssConsumeQuota(ctx, relayInfo, modelName, &dto.RealtimeUsage{
		TotalTokens:  14,
		InputTokens:  6,
		OutputTokens: 8,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens:  4,
			AudioTokens: 2,
		},
		OutputTokenDetails: dto.OutputTokenDetails{
			TextTokens:  5,
			AudioTokens: 3,
		},
	}, "")

	require.True(t, relayInfo.PriceData.MoneyActualAmountKnown)
	assert.Equal(t, int64(9_400), relayInfo.PriceData.MoneyActualAmountMicros)
	wallet := fetchMoneyBillingSessionWallet(t, 9102)
	assert.Equal(t, int64(1_990_600), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	settledToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_990_600), settledToken.RemainAmountMicros)
	assert.Equal(t, int64(9_400), settledToken.UsedAmountMicros)
}

func TestPreWssConsumeQuotaDefersMoneyBillingSessionSettlement(t *testing.T) {
	setupMoneyRuntimeQuotaTest(t)
	modelName := "money-runtime-realtime-pre"
	seedMoneyRuntimePolicy(t, modelName, moneyRuntimePolicyJSON)
	ctx, relayInfo, token := prepareMoneyRuntimeRequest(t, 9104, "money-wss-pre-token", "req-money-wss-pre", modelName, 30_000)
	usage := &dto.RealtimeUsage{
		TotalTokens:  14,
		InputTokens:  6,
		OutputTokens: 8,
		InputTokenDetails: dto.InputTokenDetails{
			TextTokens:  4,
			AudioTokens: 2,
		},
		OutputTokenDetails: dto.OutputTokenDetails{
			TextTokens:  5,
			AudioTokens: 3,
		},
	}

	require.NoError(t, PreWssConsumeQuota(ctx, relayInfo, usage))
	wallet := fetchMoneyBillingSessionWallet(t, 9104)
	assert.Equal(t, int64(1_970_000), wallet.AvailableMicros)
	assert.Equal(t, int64(30_000), wallet.FrozenMicros)
	preToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_970_000), preToken.RemainAmountMicros)
	assert.Equal(t, int64(30_000), preToken.UsedAmountMicros)

	PostWssConsumeQuota(ctx, relayInfo, modelName, usage, "")
	wallet = fetchMoneyBillingSessionWallet(t, 9104)
	assert.Equal(t, int64(1_990_600), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	settledToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_990_600), settledToken.RemainAmountMicros)
	assert.Equal(t, int64(9_400), settledToken.UsedAmountMicros)
}

func TestChargeViolationFeeUsesMoneyUsageExactAmount(t *testing.T) {
	setupMoneyRuntimeQuotaTest(t)
	modelName := "money-runtime-violation"
	seedMoneyRuntimePolicy(t, modelName, `{"schema_version":1,"profile_type":"money_usage_pricing","currency":"USD","rates":{"violation":{"amount_micros":123456,"basis":"per_unit","currency":"USD"}}}`)
	token := seedMoneyBillingSessionUserAndToken(t, 9103, "money-violation-token")
	ctx := newMoneyBillingSessionContext()
	ctx.Set("token_name", token.Name)
	ctx.Set("username", token.Name)
	ctx.Set(common.RequestIdKey, "req-money-violation")
	relayInfo := newMoneyRuntimeRelayInfo(9103, token, "req-money-violation", modelName, 0)
	apiErr := types.NewError(errors.New("violation"), types.ErrorCodeViolationFeeGrokCSAM)

	require.True(t, ChargeViolationFeeIfNeeded(ctx, relayInfo, apiErr))
	require.True(t, relayInfo.PriceData.MoneyActualAmountKnown)
	assert.Equal(t, int64(123_456), relayInfo.PriceData.MoneyActualAmountMicros)
	wallet := fetchMoneyBillingSessionWallet(t, 9103)
	assert.Equal(t, int64(1_876_544), wallet.AvailableMicros)
	settledToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_876_544), settledToken.RemainAmountMicros)
	assert.Equal(t, int64(123_456), settledToken.UsedAmountMicros)
}
