package service

import (
	"errors"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	moneypricing "github.com/QuantumNous/new-api/service/money_pricing"
)

type moneyUsageFeatureBuilder func(profile *moneypricing.MoneyUsagePricingProfile, endpointType string) moneypricing.MoneyUsageFeatures

func runtimeMoneyEndpointType(relayInfo *relaycommon.RelayInfo) string {
	if relayInfo == nil {
		return string(constant.EndpointTypeOpenAI)
	}
	if relayInfo.PriceData.MoneyEndpointType != "" {
		return relayInfo.PriceData.MoneyEndpointType
	}
	return string(constant.EndpointTypeOpenAI)
}

func loadRuntimeMoneyPricingProfile(relayInfo *relaycommon.RelayInfo) (*model.RetailPricingPolicy, *moneypricing.MoneyUsagePricingProfile, string, error) {
	if relayInfo == nil || !relayInfo.PriceData.MoneyPricingEnabled {
		return nil, nil, "", nil
	}
	endpointType := runtimeMoneyEndpointType(relayInfo)
	policy, err := model.GetEnabledRetailPricingPolicy(relayInfo.OriginModelName, relayInfo.UsingGroup, endpointType)
	if err != nil {
		return nil, nil, endpointType, err
	}
	profile, err := moneypricing.ValidateMoneyUsagePricingProfile(policy.BillingRuleJSON)
	if err != nil {
		return nil, nil, endpointType, err
	}
	return policy, profile, endpointType, nil
}

func applyRuntimeMoneyUsageQuote(relayInfo *relaycommon.RelayInfo, totalTokens int, build moneyUsageFeatureBuilder) (int, bool, error) {
	if relayInfo == nil || !relayInfo.PriceData.MoneyPricingEnabled {
		return 0, false, nil
	}
	if totalTokens == 0 {
		relayInfo.PriceData.MoneyActualAmountKnown = true
		relayInfo.PriceData.MoneyActualAmountMicros = 0
		return 0, true, nil
	}
	policy, profile, endpointType, err := loadRuntimeMoneyPricingProfile(relayInfo)
	if err != nil {
		return 0, false, err
	}
	if policy == nil || profile == nil {
		return 0, false, nil
	}
	features := build(profile, endpointType)
	quote, err := moneypricing.QuoteRetailPricingPolicy(policy, features, model.SettlementCurrency())
	if err != nil {
		return 0, false, err
	}
	applyRuntimeMoneyQuoteSnapshot(relayInfo, quote)
	return compatibilityQuotaFromMoneyAmount(quote.RetailAmountMicros), true, nil
}

func quoteRuntimeViolationFee(relayInfo *relaycommon.RelayInfo) (*moneypricing.MoneyQuote, bool, error) {
	if relayInfo == nil || !relayInfo.PriceData.MoneyPricingEnabled {
		return nil, false, nil
	}
	policy, profile, endpointType, err := loadRuntimeMoneyPricingProfile(relayInfo)
	if errors.Is(err, model.ErrMoneyPricingPolicyNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if policy == nil || profile == nil {
		return nil, false, nil
	}
	if _, ok := profile.Rates[moneypricing.UsageUnitViolation]; !ok {
		return nil, false, nil
	}
	quote, err := moneypricing.QuoteRetailPricingPolicy(policy, moneypricing.MoneyUsageFeatures{
		EndpointType:   endpointType,
		PublicModel:    relayInfo.OriginModelName,
		Group:          relayInfo.UsingGroup,
		ViolationCount: 1,
	}, model.SettlementCurrency())
	if err != nil {
		return nil, false, err
	}
	applyRuntimeMoneyQuoteSnapshot(relayInfo, quote)
	return quote, true, nil
}

func applyRuntimeMoneyQuoteSnapshot(relayInfo *relaycommon.RelayInfo, quote *moneypricing.MoneyQuote) {
	if relayInfo == nil || quote == nil {
		return
	}
	relayInfo.PriceData.MoneyActualAmountKnown = true
	relayInfo.PriceData.MoneyActualAmountMicros = quote.RetailAmountMicros
	relayInfo.PriceData.MoneyPricingActualFeaturesJSON = quote.FeaturesJSON
	relayInfo.PriceData.MoneyPricingActualUpstreamCostMicros = quote.UpstreamCostMicros
	relayInfo.PriceData.MoneySettlementCurrency = quote.SettlementCurrency
	relayInfo.PriceData.MoneyPricingVersion = quote.PricingVersion
	relayInfo.PriceData.MoneyPricingHash = quote.PricingHash
}

func fallbackRuntimeMoneyQuoteToPreconsume(relayInfo *relaycommon.RelayInfo) (int, bool) {
	if relayInfo == nil || !relayInfo.PriceData.MoneyPricingEnabled || !relayInfo.PriceData.MoneyPreConsumedAmountKnown {
		return 0, false
	}
	relayInfo.PriceData.MoneyActualAmountKnown = true
	relayInfo.PriceData.MoneyActualAmountMicros = relayInfo.PriceData.MoneyPreConsumedAmountMicros
	return compatibilityQuotaFromMoneyAmount(relayInfo.PriceData.MoneyPreConsumedAmountMicros), true
}

func compatibilityQuotaFromMoneyAmount(amountMicros int64) int {
	quota := model.MoneyMicrosToLegacyQuota(amountMicros)
	if amountMicros > 0 && quota == 0 {
		return 1
	}
	return quota
}

func injectRuntimeMoneyPricingInfo(other map[string]interface{}, relayInfo *relaycommon.RelayInfo) {
	if other == nil || relayInfo == nil || !relayInfo.PriceData.MoneyPricingEnabled {
		return
	}
	other["money_pricing"] = true
	other["money_pricing_mode"] = relayInfo.PriceData.MoneyPricingMode
	other["money_settlement_currency"] = relayInfo.PriceData.MoneySettlementCurrency
	other["money_pre_consumed_amount_micros"] = relayInfo.PriceData.MoneyPreConsumedAmountMicros
	if relayInfo.PriceData.MoneyActualAmountKnown {
		other["money_actual_amount_micros"] = relayInfo.PriceData.MoneyActualAmountMicros
	}
	if relayInfo.PriceData.MoneyPricingHash != "" {
		other["money_pricing_hash"] = relayInfo.PriceData.MoneyPricingHash
	}
}
