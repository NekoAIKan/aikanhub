package money_pricing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildMoneyUsageQuoteSumsChatTokenRates(t *testing.T) {
	profile := mustValidateMoneyUsageProfile(t, `{
		"schema_version": 1,
		"profile_type": "money_usage_pricing",
		"endpoint_type": "chat",
		"pricing_version": "chat-v1",
		"rates": {
			"input_token": {
				"amount_micros": 2000000,
				"basis": "per_million_units",
				"currency": "USD"
			},
			"output_token": {
				"amount_micros": 6000000,
				"basis": "per_million_units",
				"currency": "USD"
			}
		}
	}`)

	quote, err := BuildMoneyUsageQuote(MoneyUsageFeatures{
		EndpointType:  "chat",
		PublicModel:   "gpt-test",
		UpstreamModel: "upstream-gpt-test",
		InputTokens:   1500,
		OutputTokens:  250,
	}, profile)

	require.NoError(t, err)
	require.Equal(t, "chat", quote.EndpointType)
	require.Equal(t, "gpt-test", quote.PublicModel)
	require.Equal(t, "upstream-gpt-test", quote.UpstreamModel)
	require.Equal(t, int64(1750), quote.Quantity)
	require.Equal(t, MoneyUsageQuantitySource, quote.QuantitySource)
	require.Equal(t, "USD", quote.CostCurrency)
	require.Equal(t, int64(4500), quote.UpstreamCostMicros)
	require.Equal(t, int64(4500), quote.RetailAmountMicros)
	require.Equal(t, "chat-v1", quote.PricingVersion)
	require.NotEmpty(t, quote.PricingHash)
	require.JSONEq(t, `{
		"EndpointType": "chat",
		"PublicModel": "gpt-test",
		"UpstreamModel": "upstream-gpt-test",
		"Group": "",
		"InputTokens": 1500,
		"OutputTokens": 250,
		"CachedInputTokens": 0,
		"CacheWriteTokens": 0,
		"AudioInputTokens": 0,
		"AudioOutputTokens": 0,
		"RequestCount": 0,
		"ImageCount": 0,
		"AudioSeconds": 0,
		"ToolCallCount": 0,
		"WebSearchCount": 0,
		"FileSearchCount": 0,
		"ViolationCount": 0,
		"RawJSON": ""
	}`, quote.FeaturesJSON)
	require.Equal(t, []MoneyQuoteLineItem{
		{
			Unit:             UsageUnitInputToken,
			Quantity:         1500,
			Basis:            RateBasisPerMillionUnits,
			RateAmountMicros: 2000000,
			AmountMicros:     3000,
			Currency:         "USD",
		},
		{
			Unit:             UsageUnitOutputToken,
			Quantity:         250,
			Basis:            RateBasisPerMillionUnits,
			RateAmountMicros: 6000000,
			AmountMicros:     1500,
			Currency:         "USD",
		},
	}, quote.LineItems)
}

func TestBuildMoneyUsageQuoteSupportsFixedRequestImageAndAudioUnits(t *testing.T) {
	profile := mustValidateMoneyUsageProfile(t, `{
		"schema_version": 1,
		"profile_type": "money_usage_pricing",
		"endpoint_type": "multimodal",
		"rates": {
			"request": {"amount_micros": 10000, "basis": "per_unit", "currency": "USD"},
			"image": {"amount_micros": 250000, "basis": "per_unit", "currency": "USD"},
			"audio_second": {"amount_micros": 3000, "basis": "per_unit", "currency": "USD"},
			"audio_input_token": {"amount_micros": 4000000, "basis": "per_million_units", "currency": "USD"},
			"audio_output_token": {"amount_micros": 8000000, "basis": "per_million_units", "currency": "USD"}
		}
	}`)

	quote, err := BuildMoneyUsageQuote(MoneyUsageFeatures{
		RequestCount:      3,
		ImageCount:        2,
		AudioSeconds:      10,
		AudioInputTokens:  1000,
		AudioOutputTokens: 500,
	}, profile)

	require.NoError(t, err)
	require.Equal(t, int64(4000+4000+30000+500000+30000), quote.UpstreamCostMicros)
	require.Equal(t, int64(1515), quote.Quantity)
	require.Len(t, quote.LineItems, 5)
}

func TestBuildMoneyUsageQuoteAppliesFxBufferAndMarkupOptions(t *testing.T) {
	profile := mustValidateMoneyUsageProfile(t, `{
		"schema_version": 1,
		"profile_type": "money_usage_pricing",
		"endpoint_type": "request",
		"rates": {
			"request": {"amount_micros": 1000000, "basis": "per_unit", "currency": "CNY"}
		}
	}`)

	quote, err := BuildMoneyUsageQuote(MoneyUsageFeatures{RequestCount: 10}, profile, MoneyUsageQuoteOptions{
		SettlementCurrency: "USD",
		FxRateID:           "fx-cny-usd",
		FxRateMicros:       137000,
		FxBufferBps:        500,
		MarkupBps:          3000,
	})

	require.NoError(t, err)
	require.Equal(t, "CNY", quote.CostCurrency)
	require.Equal(t, "USD", quote.SettlementCurrency)
	require.Equal(t, int64(10_000_000), quote.UpstreamCostMicros)
	require.Equal(t, int64(1_870_050), quote.RetailAmountMicros)
	require.Equal(t, "fx-cny-usd", quote.FxRateID)
	require.Equal(t, int64(137000), quote.FxRateMicros)
	require.Equal(t, int64(500), quote.FxBufferBps)
	require.Equal(t, int64(3000), quote.MarkupBps)
	require.Equal(t, int64(431550), quote.GrossMarginMicros)
}

func TestBuildMoneyUsageQuoteSupportsToolSearchCacheAndViolationUnits(t *testing.T) {
	profile := mustValidateMoneyUsageProfile(t, `{
		"schema_version": 1,
		"profile_type": "money_usage_pricing",
		"endpoint_type": "surcharges",
		"rates": {
			"cached_input_token": {"amount_micros": 500000, "basis": "per_million_units", "currency": "USD"},
			"cache_write_token": {"amount_micros": 1000000, "basis": "per_million_units", "currency": "USD"},
			"tool_call": {"amount_micros": 1000, "basis": "per_unit", "currency": "USD"},
			"web_search_call": {"amount_micros": 10000, "basis": "per_unit", "currency": "USD"},
			"file_search_call": {"amount_micros": 2000, "basis": "per_unit", "currency": "USD"},
			"violation": {"amount_micros": 500000, "basis": "per_unit", "currency": "USD"}
		}
	}`)

	quote, err := BuildMoneyUsageQuote(MoneyUsageFeatures{
		CachedInputTokens: 2000,
		CacheWriteTokens:  3000,
		ToolCallCount:     4,
		WebSearchCount:    2,
		FileSearchCount:   3,
		ViolationCount:    1,
	}, profile)

	require.NoError(t, err)
	require.Equal(t, int64(1000+3000+4000+20000+6000+500000), quote.UpstreamCostMicros)
	require.Equal(t, int64(5010), quote.Quantity)
	require.Len(t, quote.LineItems, 6)
}

func TestBuildMoneyUsageQuoteErrorsWhenUsageRateIsMissing(t *testing.T) {
	profile := mustValidateMoneyUsageProfile(t, `{
		"schema_version": 1,
		"profile_type": "money_usage_pricing",
		"rates": {
			"input_token": {"amount_micros": 1000000, "basis": "per_million_units", "currency": "USD"}
		}
	}`)

	_, err := BuildMoneyUsageQuote(MoneyUsageFeatures{OutputTokens: 1}, profile)

	require.ErrorIs(t, err, ErrPricingRateNotFound)
}

func TestBuildMoneyUsageQuoteErrorsWhenActiveRatesUseDifferentCurrencies(t *testing.T) {
	profile := mustValidateMoneyUsageProfile(t, `{
		"schema_version": 1,
		"profile_type": "money_usage_pricing",
		"rates": {
			"input_token": {"amount_micros": 1000000, "basis": "per_million_units", "currency": "USD"},
			"request": {"amount_micros": 1000, "basis": "per_unit", "currency": "CNY"}
		}
	}`)

	_, err := BuildMoneyUsageQuote(MoneyUsageFeatures{InputTokens: 1, RequestCount: 1}, profile)

	require.ErrorIs(t, err, ErrMixedRateCurrencies)
}

func mustValidateMoneyUsageProfile(t *testing.T, raw string) *MoneyUsagePricingProfile {
	t.Helper()

	profile, err := ValidateMoneyUsagePricingProfile(raw)
	require.NoError(t, err)
	return profile
}
