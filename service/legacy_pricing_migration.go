package service

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
)

const (
	legacyPricingMigrationSource = "legacy_pricing_migration"
	legacyMoneyProfileSchema     = 1
	legacyMoneyProfileType       = "money_usage_pricing"
	legacyDefaultCurrency        = "USD"
	legacyMicrosPerUnit          = 1_000_000
	legacyUnitsPerMillion        = 1_000_000
	legacyBpsDenominator         = 10_000
)

// LegacyMoneyUsagePricingProfile is the money-profile JSON shape emitted by
// legacy quota pricing migration helpers.
type LegacyMoneyUsagePricingProfile struct {
	SchemaVersion int                        `json:"schema_version"`
	ProfileType   string                     `json:"profile_type"`
	EndpointType  string                     `json:"endpoint_type"`
	Model         string                     `json:"model"`
	Currency      string                     `json:"currency"`
	Rates         map[string]LegacyMoneyRate `json:"rates"`
	SourceNote    LegacyPricingSourceNote    `json:"source_note"`
}

type LegacyMoneyRate struct {
	AmountMicros int64  `json:"amount_micros"`
	Basis        string `json:"basis"`
	Currency     string `json:"currency"`
}

type LegacyPricingSourceNote struct {
	Source          string  `json:"source"`
	LegacyKind      string  `json:"legacy_kind"`
	ModelPrice      float64 `json:"model_price"`
	ModelRatio      float64 `json:"model_ratio"`
	CompletionRatio float64 `json:"completion_ratio"`
	GroupRatio      float64 `json:"group_ratio"`
	QuotaPerUnit    float64 `json:"quota_per_unit"`
}

// LegacyGroupRetailPolicyDraft captures the group multiplier semantics that
// legacy GroupRatio applied at runtime, without persisting any policy rows.
type LegacyGroupRetailPolicyDraft struct {
	Group         string                  `json:"group"`
	PolicyType    string                  `json:"policy_type"`
	MultiplierBps int64                   `json:"multiplier_bps"`
	DiscountBps   int64                   `json:"discount_bps"`
	MarkupBps     int64                   `json:"markup_bps"`
	SourceNote    LegacyPricingSourceNote `json:"source_note"`
}

func LegacyModelPriceToMoneyProfileJSON(model string, modelPrice float64) (string, error) {
	return LegacyModelPriceToMoneyProfileJSONWithCurrency(model, modelPrice, legacyDefaultCurrency)
}

func LegacyModelPriceToMoneyProfileJSONWithCurrency(model string, modelPrice float64, currency string) (string, error) {
	model, err := normalizeLegacyName("model", model)
	if err != nil {
		return "", err
	}
	if err := validateLegacyNumber("model price", modelPrice, true); err != nil {
		return "", err
	}
	currency = normalizeLegacyCurrency(currency)

	amountMicros, err := legacyUSDToMicros(modelPrice)
	if err != nil {
		return "", err
	}

	profile := newLegacyMoneyUsageProfile(model, "request", currency, map[string]LegacyMoneyRate{
		"request": {
			AmountMicros: amountMicros,
			Basis:        "per_unit",
			Currency:     currency,
		},
	}, LegacyPricingSourceNote{
		Source:       legacyPricingMigrationSource,
		LegacyKind:   "model_price",
		ModelPrice:   modelPrice,
		QuotaPerUnit: common.QuotaPerUnit,
	})

	return marshalLegacyMoneyUsageProfile(profile)
}

func LegacyModelRatioToMoneyProfileJSON(model string, modelRatio, completionRatio float64) (string, error) {
	return LegacyModelRatioToMoneyProfileJSONWithCurrency(model, modelRatio, completionRatio, legacyDefaultCurrency)
}

func LegacyModelRatioToMoneyProfileJSONWithCurrency(model string, modelRatio, completionRatio float64, currency string) (string, error) {
	model, err := normalizeLegacyName("model", model)
	if err != nil {
		return "", err
	}
	if err := validateLegacyNumber("model ratio", modelRatio, true); err != nil {
		return "", err
	}
	if err := validateLegacyNumber("completion ratio", completionRatio, true); err != nil {
		return "", err
	}
	if err := validateQuotaPerUnit(); err != nil {
		return "", err
	}
	currency = normalizeLegacyCurrency(currency)

	inputMicros := legacyModelRatioMicrosPerMillion(modelRatio)
	outputMicros := decimal.NewFromInt(inputMicros).Mul(decimal.NewFromFloat(completionRatio)).Round(0).IntPart()

	profile := newLegacyMoneyUsageProfile(model, "chat", currency, map[string]LegacyMoneyRate{
		"input_token": {
			AmountMicros: inputMicros,
			Basis:        "per_million_units",
			Currency:     currency,
		},
		"output_token": {
			AmountMicros: outputMicros,
			Basis:        "per_million_units",
			Currency:     currency,
		},
	}, LegacyPricingSourceNote{
		Source:          legacyPricingMigrationSource,
		LegacyKind:      "model_ratio",
		ModelRatio:      modelRatio,
		CompletionRatio: completionRatio,
		QuotaPerUnit:    common.QuotaPerUnit,
	})

	return marshalLegacyMoneyUsageProfile(profile)
}

func LegacyGroupRatioToRetailPolicyDrafts(groupRatios map[string]float64) ([]LegacyGroupRetailPolicyDraft, error) {
	groups := make([]string, 0, len(groupRatios))
	for group := range groupRatios {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	drafts := make([]LegacyGroupRetailPolicyDraft, 0, len(groups))
	for _, group := range groups {
		normalizedGroup, err := normalizeLegacyName("group", group)
		if err != nil {
			return nil, err
		}
		ratio := groupRatios[group]
		if err := validateLegacyNumber("group ratio", ratio, true); err != nil {
			return nil, fmt.Errorf("%s: %w", normalizedGroup, err)
		}

		multiplierBps := legacyRatioToBps(ratio)
		draft := LegacyGroupRetailPolicyDraft{
			Group:         normalizedGroup,
			PolicyType:    "retail_multiplier",
			MultiplierBps: multiplierBps,
			DiscountBps:   maxInt64(0, legacyBpsDenominator-multiplierBps),
			MarkupBps:     maxInt64(0, multiplierBps-legacyBpsDenominator),
			SourceNote: LegacyPricingSourceNote{
				Source:       legacyPricingMigrationSource,
				LegacyKind:   "group_ratio",
				GroupRatio:   ratio,
				QuotaPerUnit: common.QuotaPerUnit,
			},
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

func LegacyGroupRatioRetailPolicyDraftsToJSON(drafts []LegacyGroupRetailPolicyDraft) (string, error) {
	data, err := common.Marshal(drafts)
	if err != nil {
		return "", err
	}

	var decoded []LegacyGroupRetailPolicyDraft
	if err := common.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	return string(data), nil
}

func ParseLegacyMoneyUsagePricingProfileJSON(raw string) (LegacyMoneyUsagePricingProfile, error) {
	var profile LegacyMoneyUsagePricingProfile
	if err := common.Unmarshal([]byte(raw), &profile); err != nil {
		return LegacyMoneyUsagePricingProfile{}, err
	}
	return profile, nil
}

func newLegacyMoneyUsageProfile(model, endpointType, currency string, rates map[string]LegacyMoneyRate, sourceNote LegacyPricingSourceNote) LegacyMoneyUsagePricingProfile {
	return LegacyMoneyUsagePricingProfile{
		SchemaVersion: legacyMoneyProfileSchema,
		ProfileType:   legacyMoneyProfileType,
		EndpointType:  endpointType,
		Model:         model,
		Currency:      currency,
		Rates:         rates,
		SourceNote:    sourceNote,
	}
}

func marshalLegacyMoneyUsageProfile(profile LegacyMoneyUsagePricingProfile) (string, error) {
	data, err := common.Marshal(profile)
	if err != nil {
		return "", err
	}

	var decoded LegacyMoneyUsagePricingProfile
	if err := common.Unmarshal(data, &decoded); err != nil {
		return "", err
	}
	return string(data), nil
}

func legacyUSDToMicros(amount float64) (int64, error) {
	if err := validateLegacyNumber("amount", amount, true); err != nil {
		return 0, err
	}
	return decimal.NewFromFloat(amount).Mul(decimal.NewFromInt(legacyMicrosPerUnit)).Round(0).IntPart(), nil
}

func legacyModelRatioMicrosPerMillion(modelRatio float64) int64 {
	return decimal.NewFromFloat(modelRatio).
		Mul(decimal.NewFromInt(legacyUnitsPerMillion)).
		Mul(decimal.NewFromInt(legacyMicrosPerUnit)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(0).
		IntPart()
}

func legacyRatioToBps(ratio float64) int64 {
	return decimal.NewFromFloat(ratio).Mul(decimal.NewFromInt(legacyBpsDenominator)).Round(0).IntPart()
}

func normalizeLegacyName(kind, name string) (string, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return "", fmt.Errorf("%s must not be empty", kind)
	}
	return normalized, nil
}

func normalizeLegacyCurrency(currency string) string {
	normalized := strings.TrimSpace(currency)
	if normalized == "" {
		return legacyDefaultCurrency
	}
	return strings.ToUpper(normalized)
}

func validateLegacyNumber(name string, value float64, allowZero bool) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be finite", name)
	}
	if value < 0 {
		return fmt.Errorf("%s must not be negative", name)
	}
	if !allowZero && value == 0 {
		return fmt.Errorf("%s must be greater than zero", name)
	}
	return nil
}

func validateQuotaPerUnit() error {
	return validateLegacyNumber("quota per unit", common.QuotaPerUnit, false)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
