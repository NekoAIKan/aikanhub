package money_pricing

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type moneyUsagePricingProfileJSON struct {
	SchemaVersion  int                         `json:"schema_version"`
	ProfileType    string                      `json:"profile_type"`
	EndpointType   string                      `json:"endpoint_type"`
	Currency       string                      `json:"currency"`
	PricingVersion string                      `json:"pricing_version"`
	Rates          map[UsageUnit]moneyRateJSON `json:"rates"`
}

type moneyRateJSON struct {
	AmountMicros *int64 `json:"amount_micros"`
	Basis        string `json:"basis"`
	Currency     string `json:"currency"`
}

func ValidateMoneyUsagePricingProfile(raw string) (*MoneyUsagePricingProfile, error) {
	payload := []byte(raw)

	var runtimeJSON any
	if err := common.Unmarshal(payload, &runtimeJSON); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMoneyUsagePricingProfile, err)
	}
	if err := rejectForbiddenRuntimePricingFields(runtimeJSON, "$"); err != nil {
		return nil, err
	}

	var parsed moneyUsagePricingProfileJSON
	if err := common.Unmarshal(payload, &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMoneyUsagePricingProfile, err)
	}

	profile := &MoneyUsagePricingProfile{
		SchemaVersion:  parsed.SchemaVersion,
		ProfileType:    parsed.ProfileType,
		EndpointType:   strings.TrimSpace(parsed.EndpointType),
		Currency:       normalizeCurrency(parsed.Currency),
		PricingVersion: strings.TrimSpace(parsed.PricingVersion),
		Rates:          make(map[UsageUnit]MoneyRate, len(parsed.Rates)),
	}

	if parsed.SchemaVersion != MoneyUsagePricingSchemaVersion {
		return nil, fmt.Errorf("%w: schema_version must be %d", ErrInvalidMoneyUsagePricingProfile, MoneyUsagePricingSchemaVersion)
	}
	if parsed.ProfileType != ProfileTypeMoneyUsagePricing {
		return nil, fmt.Errorf("%w: profile_type must be %q", ErrInvalidMoneyUsagePricingProfile, ProfileTypeMoneyUsagePricing)
	}
	if len(parsed.Rates) == 0 {
		return nil, fmt.Errorf("%w: rates are required", ErrInvalidMoneyUsagePricingProfile)
	}
	if strings.EqualFold(profile.Currency, "credit") {
		return nil, fmt.Errorf("%w: currency must be money, not credit", ErrInvalidMoneyUsagePricingProfile)
	}

	for unit, rate := range parsed.Rates {
		if !IsSupportedUsageUnit(unit) {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedUsageUnit, unit)
		}
		if rate.AmountMicros == nil {
			return nil, fmt.Errorf("%w: rates.%s.amount_micros is required", ErrInvalidMoneyUsagePricingProfile, unit)
		}
		if *rate.AmountMicros < 0 {
			return nil, fmt.Errorf("%w: rates.%s.amount_micros must be non-negative", ErrInvalidMoneyUsagePricingProfile, unit)
		}

		basis := RateBasis(strings.TrimSpace(rate.Basis))
		if !IsSupportedRateBasis(basis) {
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedRateBasis, basis)
		}

		currency := normalizeCurrency(rate.Currency)
		if currency == "" {
			return nil, fmt.Errorf("%w: rates.%s.currency is required", ErrInvalidMoneyUsagePricingProfile, unit)
		}
		if strings.EqualFold(currency, "credit") {
			return nil, fmt.Errorf("%w: rates.%s.currency must be money, not credit", ErrInvalidMoneyUsagePricingProfile, unit)
		}

		profile.Rates[unit] = MoneyRate{
			AmountMicros: *rate.AmountMicros,
			Basis:        basis,
			Currency:     currency,
		}
	}

	return profile, nil
}

func IsSupportedUsageUnit(unit UsageUnit) bool {
	switch unit {
	case UsageUnitInputToken,
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
		UsageUnitViolation:
		return true
	default:
		return false
	}
}

func IsSupportedRateBasis(basis RateBasis) bool {
	switch basis {
	case RateBasisPerUnit, RateBasisPerMillionUnits:
		return true
	default:
		return false
	}
}

func rejectForbiddenRuntimePricingFields(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := path + "." + key
			switch {
			case strings.EqualFold(key, "quota"),
				strings.EqualFold(key, "credit_unit"),
				strings.EqualFold(key, "provider_credit"):
				return fmt.Errorf("%w: %s", ErrForbiddenRuntimePricingField, childPath)
			case strings.EqualFold(key, "currency_unit") && isCreditValue(child):
				return fmt.Errorf("%w: %s must not be credit", ErrForbiddenRuntimePricingField, childPath)
			}
			if err := rejectForbiddenRuntimePricingFields(child, childPath); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range typed {
			if err := rejectForbiddenRuntimePricingFields(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func isCreditValue(value any) bool {
	text, ok := value.(string)
	return ok && strings.EqualFold(strings.TrimSpace(text), "credit")
}

func normalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}
