package money_pricing

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

func QuoteRetailPricingPolicy(policy *model.RetailPricingPolicy, features MoneyUsageFeatures, settlementCurrency string) (*MoneyQuote, error) {
	if policy == nil {
		return nil, fmt.Errorf("%w: policy is nil", ErrInvalidMoneyUsagePricingProfile)
	}
	profile, err := ValidateMoneyUsagePricingProfile(policy.BillingRuleJSON)
	if err != nil {
		return nil, err
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

	options, err := quoteOptionsForPolicy(policy, profile, settlementCurrency)
	if err != nil {
		return nil, err
	}
	quote, err := BuildMoneyUsageQuote(features, profile, options)
	if err != nil {
		return nil, err
	}
	quote.PricingVersion = profile.PricingVersion
	quote.PricingHash = ProfileHash(profile)
	return quote, nil
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
