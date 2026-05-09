package money_pricing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type resolvedPolicyProfile struct {
	profile       *MoneyUsagePricingProfile
	source        string
	channelID     int
	upstreamModel string
}

func QuoteRetailPricingPolicy(policy *model.RetailPricingPolicy, features MoneyUsageFeatures, settlementCurrency string) (*MoneyQuote, error) {
	if policy == nil {
		return nil, fmt.Errorf("%w: policy is nil", ErrInvalidMoneyUsagePricingProfile)
	}
	if features.EndpointType == "" {
		features.EndpointType = policy.EndpointType
	}
	if features.PublicModel == "" {
		features.PublicModel = policy.PublicModel
	}
	if features.Group == "" {
		features.Group = policy.Group
	}

	candidates, err := resolvePolicyQuoteProfiles(policy, features)
	if err != nil {
		return nil, err
	}
	var selected *MoneyQuote
	for _, candidate := range candidates {
		candidateFeatures := features
		if strings.EqualFold(policy.PricingMode, model.PricingModeCostPlus) {
			candidateFeatures = featuresForProfileRates(candidateFeatures, candidate.profile)
		}
		if candidate.channelID > 0 {
			candidateFeatures.ChannelID = candidate.channelID
		}
		if candidate.upstreamModel != "" {
			candidateFeatures.UpstreamModel = candidate.upstreamModel
		}
		options, err := quoteOptionsForPolicy(policy, candidate.profile, settlementCurrency)
		if err != nil {
			return nil, err
		}
		quote, err := BuildMoneyUsageQuote(candidateFeatures, candidate.profile, options)
		if err != nil {
			return nil, err
		}
		if selected == nil || quote.RetailAmountMicros > selected.RetailAmountMicros ||
			(quote.RetailAmountMicros == selected.RetailAmountMicros && quoteSortKey(quote) < quoteSortKey(selected)) {
			selected = quote
		}
	}
	if selected == nil {
		return nil, model.ErrMoneyPricingPolicyNotFound
	}
	return selected, nil
}

func ActiveUsageUnitsForPolicy(policy *model.RetailPricingPolicy, base MoneyUsageFeatures) (map[UsageUnit]struct{}, error) {
	if policy == nil {
		return nil, fmt.Errorf("%w: policy is nil", ErrInvalidMoneyUsagePricingProfile)
	}
	if base.EndpointType == "" {
		base.EndpointType = policy.EndpointType
	}
	if base.PublicModel == "" {
		base.PublicModel = policy.PublicModel
	}
	if base.Group == "" {
		base.Group = policy.Group
	}
	candidates, err := resolvePolicyQuoteProfiles(policy, base)
	if err != nil {
		return nil, err
	}
	units := make(map[UsageUnit]struct{})
	for _, candidate := range candidates {
		for unit := range candidate.profile.Rates {
			units[unit] = struct{}{}
		}
	}
	return units, nil
}

func resolvePolicyQuoteProfiles(policy *model.RetailPricingPolicy, features MoneyUsageFeatures) ([]resolvedPolicyProfile, error) {
	switch policy.PricingMode {
	case model.PricingModeFixedRule:
		profile, err := ValidateMoneyUsagePricingProfile(policy.BillingRuleJSON)
		if err != nil {
			return nil, err
		}
		return []resolvedPolicyProfile{{profile: profile, source: "retail_policy"}}, nil
	case model.PricingModeCostPlus:
		return resolveCostPlusQuoteProfiles(policy, features)
	default:
		return nil, fmt.Errorf("%w: unsupported pricing_mode %q", ErrInvalidMoneyUsagePricingProfile, policy.PricingMode)
	}
}

func resolveCostPlusQuoteProfiles(policy *model.RetailPricingPolicy, features MoneyUsageFeatures) ([]resolvedPolicyProfile, error) {
	if features.ChannelID > 0 {
		upstreamModel := strings.TrimSpace(features.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = strings.TrimSpace(features.PublicModel)
		}
		if upstreamModel != "" {
			cost, err := model.GetEnabledChannelModelCost(features.ChannelID, upstreamModel, features.EndpointType)
			if err == nil {
				profile, profileErr := ValidateMoneyUsagePricingProfile(cost.BillingRuleJSON)
				if profileErr != nil {
					return nil, profileErr
				}
				return []resolvedPolicyProfile{{
					profile:       profile,
					source:        "channel_model_cost",
					channelID:     cost.ChannelId,
					upstreamModel: cost.UpstreamModel,
				}}, nil
			}
			if !errors.Is(err, model.ErrMoneyPricingPolicyNotFound) {
				return nil, err
			}
		}
		if strings.TrimSpace(policy.BillingRuleJSON) == "" {
			return nil, model.ErrMoneyPricingPolicyNotFound
		}
		profile, err := ValidateMoneyUsagePricingProfile(policy.BillingRuleJSON)
		if err != nil {
			return nil, err
		}
		return []resolvedPolicyProfile{{profile: profile, source: "retail_policy_fallback"}}, nil
	}

	if strings.TrimSpace(policy.BillingRuleJSON) != "" {
		profile, err := ValidateMoneyUsagePricingProfile(policy.BillingRuleJSON)
		if err != nil {
			return nil, err
		}
		return []resolvedPolicyProfile{{profile: profile, source: "retail_policy_fallback"}}, nil
	}

	costs, err := model.ListEnabledChannelModelCostsForPublicModel(features.PublicModel, features.Group, features.EndpointType)
	if err != nil {
		return nil, err
	}
	candidates := make([]resolvedPolicyProfile, 0, len(costs))
	for _, cost := range costs {
		profile, err := ValidateMoneyUsagePricingProfile(cost.BillingRuleJSON)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, resolvedPolicyProfile{
			profile:       profile,
			source:        "channel_model_cost",
			channelID:     cost.ChannelId,
			upstreamModel: cost.UpstreamModel,
		})
	}
	if len(candidates) == 0 {
		return nil, model.ErrMoneyPricingPolicyNotFound
	}
	return candidates, nil
}

func featuresForProfileRates(features MoneyUsageFeatures, profile *MoneyUsagePricingProfile) MoneyUsageFeatures {
	if profile == nil {
		return features
	}
	if _, ok := profile.Rates[UsageUnitInputToken]; !ok {
		features.InputTokens = 0
	}
	if _, ok := profile.Rates[UsageUnitOutputToken]; !ok {
		features.OutputTokens = 0
	}
	if _, ok := profile.Rates[UsageUnitCachedInputToken]; !ok {
		features.CachedInputTokens = 0
	}
	if _, ok := profile.Rates[UsageUnitCacheWriteToken]; !ok {
		features.CacheWriteTokens = 0
	}
	if _, ok := profile.Rates[UsageUnitAudioInputToken]; !ok {
		features.AudioInputTokens = 0
	}
	if _, ok := profile.Rates[UsageUnitAudioOutputToken]; !ok {
		features.AudioOutputTokens = 0
	}
	if _, ok := profile.Rates[UsageUnitRequest]; !ok {
		features.RequestCount = 0
	}
	if _, ok := profile.Rates[UsageUnitImage]; !ok {
		features.ImageCount = 0
	}
	if _, ok := profile.Rates[UsageUnitAudioSecond]; !ok {
		features.AudioSeconds = 0
	}
	if _, ok := profile.Rates[UsageUnitToolCall]; !ok {
		features.ToolCallCount = 0
	}
	if _, ok := profile.Rates[UsageUnitWebSearchCall]; !ok {
		features.WebSearchCount = 0
	}
	if _, ok := profile.Rates[UsageUnitFileSearchCall]; !ok {
		features.FileSearchCount = 0
	}
	if _, ok := profile.Rates[UsageUnitViolation]; !ok {
		features.ViolationCount = 0
	}
	return features
}

func quoteSortKey(quote *MoneyQuote) string {
	if quote == nil {
		return ""
	}
	return fmt.Sprintf("%020d:%s", quote.ChannelID, quote.UpstreamModel)
}

func quoteOptionsForPolicy(policy *model.RetailPricingPolicy, profile *MoneyUsagePricingProfile, settlementCurrency string) (MoneyUsageQuoteOptions, error) {
	costCurrency := firstRateCurrency(profile)
	if costCurrency == "" {
		costCurrency = normalizeCurrency(policy.Currency)
	}
	if costCurrency == "" {
		costCurrency = normalizeCurrency(profile.Currency)
	}
	settlement := normalizeCurrency(settlementCurrency)
	if settlement == "" {
		settlement = costCurrency
	}

	options := MoneyUsageQuoteOptions{
		SettlementCurrency: settlement,
		FxRateID:           "same:" + costCurrency,
		FxRateMicros:       millionUnits,
	}
	if strings.EqualFold(policy.PricingMode, model.PricingModeCostPlus) {
		options.MarkupBps = policy.MarkupBps
		options.FxBufferBps = policy.FxBufferBps
	}
	if costCurrency == settlement {
		if options.FxBufferBps > 0 {
			options.FxRateID = "same:" + costCurrency
		}
		return options, nil
	}

	rate, err := model.GetLatestFxRate(costCurrency, settlement)
	if err != nil {
		return MoneyUsageQuoteOptions{}, err
	}
	options.FxRateID = rate.Id
	options.FxRateMicros = rate.RateMicros
	options.FxBufferBps += rate.BufferBps
	return options, nil
}

func firstRateCurrency(profile *MoneyUsagePricingProfile) string {
	if profile == nil {
		return ""
	}
	for _, rate := range profile.Rates {
		if currency := normalizeCurrency(rate.Currency); currency != "" {
			return currency
		}
	}
	return normalizeCurrency(profile.Currency)
}
