package money_pricing

import "errors"

const (
	MoneyUsagePricingSchemaVersion = 1
	ProfileTypeMoneyUsagePricing   = "money_usage_pricing"

	RateBasisPerUnit         RateBasis = "per_unit"
	RateBasisPerMillionUnits RateBasis = "per_million_units"

	UsageUnitInputToken       UsageUnit = "input_token"
	UsageUnitOutputToken      UsageUnit = "output_token"
	UsageUnitCachedInputToken UsageUnit = "cached_input_token"
	UsageUnitCacheWriteToken  UsageUnit = "cache_write_token"
	UsageUnitAudioInputToken  UsageUnit = "audio_input_token"
	UsageUnitAudioOutputToken UsageUnit = "audio_output_token"
	UsageUnitRequest          UsageUnit = "request"
	UsageUnitImage            UsageUnit = "image"
	UsageUnitAudioSecond      UsageUnit = "audio_second"
	UsageUnitToolCall         UsageUnit = "tool_call"
	UsageUnitWebSearchCall    UsageUnit = "web_search_call"
	UsageUnitFileSearchCall   UsageUnit = "file_search_call"
	UsageUnitViolation        UsageUnit = "violation"

	MoneyUsageQuantitySource = "money_usage_pricing"
)

var (
	ErrInvalidMoneyUsagePricingProfile = errors.New("invalid money usage pricing profile")
	ErrForbiddenRuntimePricingField    = errors.New("forbidden runtime pricing field")
	ErrUnsupportedUsageUnit            = errors.New("unsupported money usage unit")
	ErrUnsupportedRateBasis            = errors.New("unsupported money rate basis")
	ErrPricingRateNotFound             = errors.New("pricing rate not found")
	ErrMixedRateCurrencies             = errors.New("mixed rate currencies")
	ErrInvalidMoneyUsageFeatures       = errors.New("invalid money usage features")
	ErrMoneyAmountOverflow             = errors.New("money amount overflow")
)

type UsageUnit string

type RateBasis string

type MoneyUsageFeatures struct {
	EndpointType      string
	PublicModel       string
	UpstreamModel     string
	ChannelID         int `json:"ChannelID,omitempty"`
	Group             string
	InputTokens       int64
	OutputTokens      int64
	CachedInputTokens int64
	CacheWriteTokens  int64
	AudioInputTokens  int64
	AudioOutputTokens int64
	RequestCount      int64
	ImageCount        int64
	AudioSeconds      int64
	ToolCallCount     int64
	WebSearchCount    int64
	FileSearchCount   int64
	ViolationCount    int64
	RawJSON           string
}

type MoneyUsagePricingProfile struct {
	SchemaVersion  int                     `json:"schema_version"`
	ProfileType    string                  `json:"profile_type"`
	EndpointType   string                  `json:"endpoint_type,omitempty"`
	Currency       string                  `json:"currency,omitempty"`
	PricingVersion string                  `json:"pricing_version,omitempty"`
	Rates          map[UsageUnit]MoneyRate `json:"rates"`
}

type MoneyRate struct {
	AmountMicros int64     `json:"amount_micros"`
	Basis        RateBasis `json:"basis"`
	Currency     string    `json:"currency"`
}

type MoneyQuote struct {
	EndpointType       string
	PublicModel        string
	UpstreamModel      string
	ChannelID          int
	RuleID             string
	AxesKey            string
	Quantity           int64
	QuantitySource     string
	CostCurrency       string
	UpstreamCostMicros int64
	SettlementCurrency string
	RetailAmountMicros int64
	FxRateID           string
	FxRateMicros       int64
	FxBufferBps        int64
	MarkupBps          int64
	GrossMarginMicros  int64
	PricingVersion     string
	PricingHash        string
	FeaturesJSON       string
	LineItems          []MoneyQuoteLineItem
}

type MoneyQuoteLineItem struct {
	Unit             UsageUnit
	Quantity         int64
	Basis            RateBasis
	RateAmountMicros int64
	AmountMicros     int64
	Currency         string
}

type MoneyUsageQuoteOptions struct {
	SettlementCurrency string
	FxRateID           string
	FxRateMicros       int64
	FxBufferBps        int64
	MarkupBps          int64
}
