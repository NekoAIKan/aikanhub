package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	PricingModeCostPlus  = "cost_plus"
	PricingModeFixedRule = "fixed_rule"
	DefaultPricingGroup  = "default"
)

var (
	ErrMoneyPricingPolicyNotFound = errors.New("money pricing policy not found")
	ErrMoneyPricingProfileInvalid = errors.New("money pricing profile invalid")
)

type ChannelModelCost struct {
	Id              int    `json:"id"`
	ChannelId       int    `json:"channel_id" gorm:"index:idx_channel_model_cost_lookup,priority:1;not null"`
	UpstreamModel   string `json:"upstream_model" gorm:"type:varchar(255);index:idx_channel_model_cost_lookup,priority:2;not null"`
	EndpointType    string `json:"endpoint_type" gorm:"type:varchar(64);index:idx_channel_model_cost_lookup,priority:3;not null"`
	BillingRuleJSON string `json:"billing_rule_json" gorm:"type:text;not null"`
	Source          string `json:"source" gorm:"type:varchar(32);index;not null"`
	SourceRef       string `json:"source_ref" gorm:"type:varchar(255)"`
	Enabled         bool   `json:"enabled" gorm:"index;not null;default:true"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

type RetailPricingPolicy struct {
	Id              int    `json:"id"`
	PublicModel     string `json:"public_model" gorm:"type:varchar(255);index:idx_retail_pricing_policy_lookup,priority:1;not null"`
	Group           string `json:"group" gorm:"column:group;type:varchar(64);index:idx_retail_pricing_policy_lookup,priority:2;not null;default:'default'"`
	EndpointType    string `json:"endpoint_type" gorm:"type:varchar(64);index:idx_retail_pricing_policy_lookup,priority:3;not null"`
	PricingMode     string `json:"pricing_mode" gorm:"type:varchar(32);not null"`
	BillingRuleJSON string `json:"billing_rule_json" gorm:"type:text"`
	MarkupBps       int64  `json:"markup_bps" gorm:"bigint;not null;default:0"`
	FxPolicy        string `json:"fx_policy" gorm:"type:varchar(32);not null;default:'latest'"`
	FxBufferBps     int64  `json:"fx_buffer_bps" gorm:"bigint;not null;default:0"`
	Currency        string `json:"currency" gorm:"type:varchar(8);not null"`
	Enabled         bool   `json:"enabled" gorm:"index;not null;default:true"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

func (cost *ChannelModelCost) BeforeSave(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if cost.CreatedAt == 0 {
		cost.CreatedAt = now
	}
	cost.UpdatedAt = now
	cost.UpstreamModel = strings.TrimSpace(cost.UpstreamModel)
	cost.EndpointType = strings.TrimSpace(cost.EndpointType)
	cost.Source = strings.TrimSpace(cost.Source)
	if cost.ChannelId <= 0 || cost.UpstreamModel == "" || cost.EndpointType == "" || cost.Source == "" {
		return fmt.Errorf("%w: channel_id, upstream_model, endpoint_type, and source are required", ErrMoneyPricingProfileInvalid)
	}
	return validateMoneyPricingProfileJSON(cost.BillingRuleJSON, true)
}

func (policy *RetailPricingPolicy) BeforeSave(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if policy.CreatedAt == 0 {
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now
	policy.PublicModel = strings.TrimSpace(policy.PublicModel)
	policy.Group = strings.TrimSpace(policy.Group)
	if policy.Group == "" {
		policy.Group = DefaultPricingGroup
	}
	policy.EndpointType = strings.TrimSpace(policy.EndpointType)
	policy.PricingMode = strings.TrimSpace(policy.PricingMode)
	policy.FxPolicy = strings.TrimSpace(policy.FxPolicy)
	if policy.FxPolicy == "" {
		policy.FxPolicy = "latest"
	}
	policy.Currency = strings.ToUpper(strings.TrimSpace(policy.Currency))
	if policy.PublicModel == "" || policy.EndpointType == "" || policy.PricingMode == "" || policy.Currency == "" {
		return fmt.Errorf("%w: public_model, endpoint_type, pricing_mode, and currency are required", ErrMoneyPricingProfileInvalid)
	}
	switch policy.PricingMode {
	case PricingModeCostPlus:
		if strings.TrimSpace(policy.BillingRuleJSON) != "" {
			return validateMoneyPricingProfileJSON(policy.BillingRuleJSON, true)
		}
		return nil
	case PricingModeFixedRule:
		return validateMoneyPricingProfileJSON(policy.BillingRuleJSON, true)
	default:
		return fmt.Errorf("%w: unsupported pricing_mode %q", ErrMoneyPricingProfileInvalid, policy.PricingMode)
	}
}

func validateMoneyPricingProfileJSON(raw string, required bool) error {
	if strings.TrimSpace(raw) == "" {
		if required {
			return fmt.Errorf("%w: billing_rule_json is required", ErrMoneyPricingProfileInvalid)
		}
		return nil
	}
	var payload map[string]any
	if err := common.Unmarshal([]byte(raw), &payload); err != nil {
		return fmt.Errorf("%w: %v", ErrMoneyPricingProfileInvalid, err)
	}
	if len(payload) == 0 {
		return fmt.Errorf("%w: billing_rule_json must be a non-empty JSON object", ErrMoneyPricingProfileInvalid)
	}
	if err := rejectForbiddenMoneyPricingFields(payload, "$"); err != nil {
		return err
	}
	profileType, _ := payload["profile_type"].(string)
	switch profileType {
	case "money_usage_pricing":
		if err := validateMoneyUsagePricingProfilePayload(payload); err != nil {
			return err
		}
	case "video_rule_matrix":
		if _, ok := payload["schema_version"].(float64); !ok {
			return fmt.Errorf("%w: schema_version is required", ErrMoneyPricingProfileInvalid)
		}
		if _, ok := payload["rules"].([]any); !ok {
			return fmt.Errorf("%w: video rules are required", ErrMoneyPricingProfileInvalid)
		}
	default:
		return fmt.Errorf("%w: unsupported profile_type %q", ErrMoneyPricingProfileInvalid, profileType)
	}
	return nil
}

func rejectForbiddenMoneyPricingFields(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := path + "." + key
			switch {
			case strings.EqualFold(key, "quota"),
				strings.EqualFold(key, "credit_unit"),
				strings.EqualFold(key, "provider_credit"):
				return fmt.Errorf("%w: forbidden field %s", ErrMoneyPricingProfileInvalid, childPath)
			case strings.EqualFold(key, "currency_unit") && isCreditMoneyPricingValue(child):
				return fmt.Errorf("%w: forbidden credit currency unit at %s", ErrMoneyPricingProfileInvalid, childPath)
			}
			if err := rejectForbiddenMoneyPricingFields(child, childPath); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range typed {
			if err := rejectForbiddenMoneyPricingFields(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func isCreditMoneyPricingValue(value any) bool {
	text, ok := value.(string)
	return ok && strings.EqualFold(strings.TrimSpace(text), "credit")
}

func validateMoneyUsagePricingProfilePayload(payload map[string]any) error {
	version, ok := payload["schema_version"].(float64)
	if !ok || version != 1 || math.Trunc(version) != version {
		return fmt.Errorf("%w: schema_version must be 1", ErrMoneyPricingProfileInvalid)
	}
	rates, ok := payload["rates"].(map[string]any)
	if !ok || len(rates) == 0 {
		return fmt.Errorf("%w: rates are required", ErrMoneyPricingProfileInvalid)
	}
	for unit, rawRate := range rates {
		if !isSupportedMoneyUsageUnit(unit) {
			return fmt.Errorf("%w: unsupported rate unit %s", ErrMoneyPricingProfileInvalid, unit)
		}
		rate, ok := rawRate.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: rate %s must be an object", ErrMoneyPricingProfileInvalid, unit)
		}
		amountMicros, ok := rate["amount_micros"].(float64)
		if !ok {
			return fmt.Errorf("%w: rate %s amount_micros is required", ErrMoneyPricingProfileInvalid, unit)
		}
		if amountMicros < 0 || math.Trunc(amountMicros) != amountMicros {
			return fmt.Errorf("%w: rate %s amount_micros must be a non-negative integer", ErrMoneyPricingProfileInvalid, unit)
		}
		basis, _ := rate["basis"].(string)
		if basis != "per_unit" && basis != "per_million_units" {
			return fmt.Errorf("%w: rate %s has unsupported basis", ErrMoneyPricingProfileInvalid, unit)
		}
		currency, _ := rate["currency"].(string)
		if strings.TrimSpace(currency) == "" || strings.EqualFold(currency, "credit") {
			return fmt.Errorf("%w: rate %s currency must be money", ErrMoneyPricingProfileInvalid, unit)
		}
	}
	return nil
}

func isSupportedMoneyUsageUnit(unit string) bool {
	switch unit {
	case "input_token",
		"output_token",
		"cached_input_token",
		"cache_write_token",
		"audio_input_token",
		"audio_output_token",
		"request",
		"image",
		"audio_second",
		"tool_call",
		"web_search_call",
		"file_search_call",
		"violation":
		return true
	default:
		return false
	}
}

func GetEnabledChannelModelCost(channelID int, upstreamModel string, endpointType string) (*ChannelModelCost, error) {
	var cost ChannelModelCost
	err := DB.Where("channel_id = ? AND upstream_model = ? AND endpoint_type = ? AND enabled = ?", channelID, strings.TrimSpace(upstreamModel), strings.TrimSpace(endpointType), true).
		Order("id DESC").
		First(&cost).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMoneyPricingPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cost, nil
}

func GetEnabledRetailPricingPolicy(publicModel string, group string, endpointType string) (*RetailPricingPolicy, error) {
	publicModel = strings.TrimSpace(publicModel)
	group = strings.TrimSpace(group)
	if group == "" {
		group = DefaultPricingGroup
	}
	endpointType = strings.TrimSpace(endpointType)
	policy, err := getEnabledRetailPricingPolicyExact(publicModel, group, endpointType)
	if err == nil {
		return policy, nil
	}
	if !errors.Is(err, ErrMoneyPricingPolicyNotFound) || group == DefaultPricingGroup {
		return nil, err
	}
	return getEnabledRetailPricingPolicyExact(publicModel, DefaultPricingGroup, endpointType)
}

func getEnabledRetailPricingPolicyExact(publicModel string, group string, endpointType string) (*RetailPricingPolicy, error) {
	var policy RetailPricingPolicy
	err := DB.Where(fmt.Sprintf("public_model = ? AND %s = ? AND endpoint_type = ? AND enabled = ?", retailPricingGroupColumn()), publicModel, group, endpointType, true).
		Order("id DESC").
		First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMoneyPricingPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func retailPricingGroupColumn() string {
	if commonGroupCol != "" {
		return commonGroupCol
	}
	if common.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}
