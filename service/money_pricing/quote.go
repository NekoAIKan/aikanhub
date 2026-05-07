package money_pricing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const millionUnits = int64(1_000_000)

type usageQuantity struct {
	unit     UsageUnit
	quantity int64
}

func BuildMoneyUsageQuote(features MoneyUsageFeatures, profile *MoneyUsagePricingProfile, options ...MoneyUsageQuoteOptions) (*MoneyQuote, error) {
	if err := validateProfileForQuote(profile); err != nil {
		return nil, err
	}

	var totalQuantity int64
	var totalAmountMicros int64
	var currency string
	lineItems := make([]MoneyQuoteLineItem, 0)

	for _, usage := range usageQuantities(features) {
		if usage.quantity < 0 {
			return nil, fmt.Errorf("%w: %s quantity must be non-negative", ErrInvalidMoneyUsageFeatures, usage.unit)
		}
		if usage.quantity == 0 {
			continue
		}

		rate, ok := profile.Rates[usage.unit]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrPricingRateNotFound, usage.unit)
		}
		rate.Currency = normalizeCurrency(rate.Currency)
		if currency == "" {
			currency = rate.Currency
		} else if currency != rate.Currency {
			return nil, fmt.Errorf("%w: %s and %s", ErrMixedRateCurrencies, currency, rate.Currency)
		}

		amountMicros, err := calculateRateAmountMicros(rate, usage.quantity)
		if err != nil {
			return nil, err
		}
		totalQuantity, err = checkedAdd(totalQuantity, usage.quantity)
		if err != nil {
			return nil, err
		}
		totalAmountMicros, err = checkedAdd(totalAmountMicros, amountMicros)
		if err != nil {
			return nil, err
		}

		lineItems = append(lineItems, MoneyQuoteLineItem{
			Unit:             usage.unit,
			Quantity:         usage.quantity,
			Basis:            rate.Basis,
			RateAmountMicros: rate.AmountMicros,
			AmountMicros:     amountMicros,
			Currency:         rate.Currency,
		})
	}

	if currency == "" {
		currency = firstProfileCurrency(profile)
	}
	quoteOptions := resolveMoneyUsageQuoteOptions(currency, options)
	settlementCostMicros, retailAmountMicros, err := applyMoneyUsageQuoteOptions(totalAmountMicros, quoteOptions)
	if err != nil {
		return nil, err
	}
	settlementCurrency := normalizeCurrency(quoteOptions.SettlementCurrency)

	featuresJSON, err := common.Marshal(features)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal features: %v", ErrInvalidMoneyUsageFeatures, err)
	}

	endpointType := features.EndpointType
	if endpointType == "" {
		endpointType = profile.EndpointType
	}

	return &MoneyQuote{
		EndpointType:       endpointType,
		PublicModel:        features.PublicModel,
		UpstreamModel:      features.UpstreamModel,
		RuleID:             ProfileTypeMoneyUsagePricing,
		Quantity:           totalQuantity,
		QuantitySource:     MoneyUsageQuantitySource,
		CostCurrency:       currency,
		UpstreamCostMicros: totalAmountMicros,
		SettlementCurrency: settlementCurrency,
		RetailAmountMicros: retailAmountMicros,
		FxRateID:           quoteOptions.FxRateID,
		FxRateMicros:       quoteOptions.FxRateMicros,
		FxBufferBps:        quoteOptions.FxBufferBps,
		MarkupBps:          quoteOptions.MarkupBps,
		GrossMarginMicros:  retailAmountMicros - settlementCostMicros,
		PricingVersion:     profile.PricingVersion,
		PricingHash:        ProfileHash(profile),
		FeaturesJSON:       string(featuresJSON),
		LineItems:          lineItems,
	}, nil
}

func resolveMoneyUsageQuoteOptions(costCurrency string, options []MoneyUsageQuoteOptions) MoneyUsageQuoteOptions {
	resolved := MoneyUsageQuoteOptions{
		SettlementCurrency: costCurrency,
		FxRateID:           "same:" + costCurrency,
		FxRateMicros:       millionUnits,
	}
	if len(options) > 0 {
		resolved = options[0]
	}
	resolved.SettlementCurrency = normalizeCurrency(resolved.SettlementCurrency)
	if resolved.SettlementCurrency == "" {
		resolved.SettlementCurrency = costCurrency
	}
	if resolved.FxRateMicros == 0 {
		resolved.FxRateMicros = millionUnits
	}
	if resolved.FxRateID == "" && resolved.SettlementCurrency == costCurrency && resolved.FxRateMicros == millionUnits {
		resolved.FxRateID = "same:" + costCurrency
	}
	return resolved
}

func applyMoneyUsageQuoteOptions(amountMicros int64, options MoneyUsageQuoteOptions) (int64, int64, error) {
	converted, err := checkedMul(amountMicros, options.FxRateMicros)
	if err != nil {
		return 0, 0, err
	}
	converted = ceilDiv(converted, millionUnits)
	converted = applyBps(converted, options.FxBufferBps)
	retail := applyBps(converted, options.MarkupBps)
	return converted, retail, nil
}

func applyBps(amountMicros int64, bps int64) int64 {
	if amountMicros == 0 || bps == 0 {
		return amountMicros
	}
	return amountMicros * (10_000 + bps) / 10_000
}

func ProfileHash(profile *MoneyUsagePricingProfile) string {
	if profile == nil {
		return ""
	}
	payload, err := common.Marshal(profile)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func validateProfileForQuote(profile *MoneyUsagePricingProfile) error {
	if profile == nil {
		return fmt.Errorf("%w: profile is nil", ErrInvalidMoneyUsagePricingProfile)
	}
	if profile.SchemaVersion != MoneyUsagePricingSchemaVersion {
		return fmt.Errorf("%w: schema_version must be %d", ErrInvalidMoneyUsagePricingProfile, MoneyUsagePricingSchemaVersion)
	}
	if profile.ProfileType != ProfileTypeMoneyUsagePricing {
		return fmt.Errorf("%w: profile_type must be %q", ErrInvalidMoneyUsagePricingProfile, ProfileTypeMoneyUsagePricing)
	}
	if len(profile.Rates) == 0 {
		return fmt.Errorf("%w: rates are required", ErrInvalidMoneyUsagePricingProfile)
	}
	if strings.EqualFold(profile.Currency, "credit") {
		return fmt.Errorf("%w: currency must be money, not credit", ErrInvalidMoneyUsagePricingProfile)
	}
	for unit, rate := range profile.Rates {
		if !IsSupportedUsageUnit(unit) {
			return fmt.Errorf("%w: %s", ErrUnsupportedUsageUnit, unit)
		}
		if rate.AmountMicros < 0 {
			return fmt.Errorf("%w: rates.%s.amount_micros must be non-negative", ErrInvalidMoneyUsagePricingProfile, unit)
		}
		if !IsSupportedRateBasis(rate.Basis) {
			return fmt.Errorf("%w: %s", ErrUnsupportedRateBasis, rate.Basis)
		}
		currency := normalizeCurrency(rate.Currency)
		if currency == "" {
			return fmt.Errorf("%w: rates.%s.currency is required", ErrInvalidMoneyUsagePricingProfile, unit)
		}
		if strings.EqualFold(currency, "credit") {
			return fmt.Errorf("%w: rates.%s.currency must be money, not credit", ErrInvalidMoneyUsagePricingProfile, unit)
		}
	}
	return nil
}

func usageQuantities(features MoneyUsageFeatures) []usageQuantity {
	return []usageQuantity{
		{unit: UsageUnitInputToken, quantity: features.InputTokens},
		{unit: UsageUnitOutputToken, quantity: features.OutputTokens},
		{unit: UsageUnitCachedInputToken, quantity: features.CachedInputTokens},
		{unit: UsageUnitCacheWriteToken, quantity: features.CacheWriteTokens},
		{unit: UsageUnitAudioInputToken, quantity: features.AudioInputTokens},
		{unit: UsageUnitAudioOutputToken, quantity: features.AudioOutputTokens},
		{unit: UsageUnitRequest, quantity: features.RequestCount},
		{unit: UsageUnitImage, quantity: features.ImageCount},
		{unit: UsageUnitAudioSecond, quantity: features.AudioSeconds},
		{unit: UsageUnitToolCall, quantity: features.ToolCallCount},
		{unit: UsageUnitWebSearchCall, quantity: features.WebSearchCount},
		{unit: UsageUnitFileSearchCall, quantity: features.FileSearchCount},
		{unit: UsageUnitViolation, quantity: features.ViolationCount},
	}
}

func calculateRateAmountMicros(rate MoneyRate, quantity int64) (int64, error) {
	product, err := checkedMul(rate.AmountMicros, quantity)
	if err != nil {
		return 0, err
	}

	switch rate.Basis {
	case RateBasisPerUnit:
		return product, nil
	case RateBasisPerMillionUnits:
		return ceilDiv(product, millionUnits), nil
	default:
		return 0, fmt.Errorf("%w: %s", ErrUnsupportedRateBasis, rate.Basis)
	}
}

func checkedMul(left int64, right int64) (int64, error) {
	if left < 0 || right < 0 {
		return 0, ErrInvalidMoneyUsageFeatures
	}
	if left != 0 && right > math.MaxInt64/left {
		return 0, ErrMoneyAmountOverflow
	}
	return left * right, nil
}

func checkedAdd(left int64, right int64) (int64, error) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, ErrMoneyAmountOverflow
	}
	return left + right, nil
}

func ceilDiv(numerator int64, denominator int64) int64 {
	if numerator == 0 {
		return 0
	}
	return (numerator-1)/denominator + 1
}

func firstProfileCurrency(profile *MoneyUsagePricingProfile) string {
	if profile.Currency != "" {
		return normalizeCurrency(profile.Currency)
	}
	for _, unit := range []UsageUnit{
		UsageUnitInputToken,
		UsageUnitOutputToken,
		UsageUnitCachedInputToken,
		UsageUnitCacheWriteToken,
		UsageUnitAudioInputToken,
		UsageUnitAudioOutputToken,
		UsageUnitRequest,
		UsageUnitImage,
		UsageUnitAudioSecond,
		UsageUnitToolCall,
		UsageUnitWebSearchCall,
		UsageUnitFileSearchCall,
		UsageUnitViolation,
	} {
		if rate, ok := profile.Rates[unit]; ok {
			return normalizeCurrency(rate.Currency)
		}
	}
	return ""
}
