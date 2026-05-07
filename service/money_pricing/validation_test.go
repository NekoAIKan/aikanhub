package money_pricing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateMoneyUsagePricingProfileAcceptsExplicitMoneyRates(t *testing.T) {
	raw := `{
		"schema_version": 1,
		"profile_type": "money_usage_pricing",
		"endpoint_type": "chat",
		"rates": {
			"input_token": {
				"amount_micros": 2000000,
				"basis": "per_million_units",
				"currency": "USD"
			},
			"request": {
				"amount_micros": 10000,
				"basis": "per_unit",
				"currency": "USD"
			}
		}
	}`

	profile, err := ValidateMoneyUsagePricingProfile(raw)

	require.NoError(t, err)
	require.Equal(t, 1, profile.SchemaVersion)
	require.Equal(t, ProfileTypeMoneyUsagePricing, profile.ProfileType)
	require.Equal(t, "chat", profile.EndpointType)
	require.Equal(t, int64(2000000), profile.Rates[UsageUnitInputToken].AmountMicros)
	require.Equal(t, RateBasisPerMillionUnits, profile.Rates[UsageUnitInputToken].Basis)
	require.Equal(t, "USD", profile.Rates[UsageUnitInputToken].Currency)
}

func TestValidateMoneyUsagePricingProfileRequiresSchemaVersionOneAndProfileType(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing schema version",
			raw: `{
				"profile_type": "money_usage_pricing",
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
		{
			name: "wrong schema version",
			raw: `{
				"schema_version": 2,
				"profile_type": "money_usage_pricing",
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
		{
			name: "wrong profile type",
			raw: `{
				"schema_version": 1,
				"profile_type": "video_rule_matrix",
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateMoneyUsagePricingProfile(tt.raw)
			require.Error(t, err)
		})
	}
}

func TestValidateMoneyUsagePricingProfileRejectsQuotaAndCreditFieldsAnywhere(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "quota field",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"metadata": {"quota": 42},
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
		{
			name: "credit unit field",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"rates": {
					"request": {
						"amount_micros": 1,
						"basis": "per_unit",
						"currency": "USD",
						"credit_unit": "pixverse_credit"
					}
				}
			}`,
		},
		{
			name: "provider credit field",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"source_note": {"provider_credit": "200 credits per USD"},
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
		{
			name: "currency unit credit value in array",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"notes": [{"currency_unit": "credit"}],
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateMoneyUsagePricingProfile(tt.raw)
			require.Error(t, err)
		})
	}
}

func TestValidateMoneyUsagePricingProfileRequiresExplicitSupportedMoneyRates(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing amount",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"rates": {
					"request": {"basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
		{
			name: "missing basis",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"rates": {
					"request": {"amount_micros": 1, "currency": "USD"}
				}
			}`,
		},
		{
			name: "missing currency",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_unit"}
				}
			}`,
		},
		{
			name: "unsupported basis",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"rates": {
					"request": {"amount_micros": 1, "basis": "per_credit", "currency": "USD"}
				}
			}`,
		},
		{
			name: "unsupported unit",
			raw: `{
				"schema_version": 1,
				"profile_type": "money_usage_pricing",
				"rates": {
					"unknown_unit": {"amount_micros": 1, "basis": "per_unit", "currency": "USD"}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateMoneyUsagePricingProfile(tt.raw)
			require.Error(t, err)
		})
	}
}
