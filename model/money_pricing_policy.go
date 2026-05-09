package model

import (
	"errors"
	"fmt"
	"math"
	"sort"
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

type RetailPricingPolicyLookup map[string]*RetailPricingPolicy

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

func ListEnabledChannelModelCostsForPublicModel(publicModel string, group string, endpointType string) ([]ChannelModelCost, error) {
	publicModel = strings.TrimSpace(publicModel)
	group = strings.TrimSpace(group)
	if group == "" {
		group = DefaultPricingGroup
	}
	endpointType = strings.TrimSpace(endpointType)
	if publicModel == "" || endpointType == "" {
		return nil, ErrMoneyPricingPolicyNotFound
	}

	var abilities []Ability
	if err := DB.Where(fmt.Sprintf("%s = ? AND model = ? AND enabled = ?", retailPricingGroupColumn()), group, publicModel, true).
		Find(&abilities).Error; err != nil {
		return nil, err
	}
	if len(abilities) == 0 && group != DefaultPricingGroup {
		if err := DB.Where(fmt.Sprintf("%s = ? AND model = ? AND enabled = ?", retailPricingGroupColumn()), DefaultPricingGroup, publicModel, true).
			Find(&abilities).Error; err != nil {
			return nil, err
		}
	}
	if len(abilities) == 0 {
		return nil, ErrMoneyPricingPolicyNotFound
	}

	channelIDs := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		channelIDs = append(channelIDs, ability.ChannelId)
	}

	var channels []Channel
	if err := DB.Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	channelByID := make(map[int]Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
	}

	var costs []ChannelModelCost
	if err := DB.Where("channel_id IN ? AND endpoint_type = ? AND enabled = ?", channelIDs, endpointType, true).
		Order("channel_id ASC, id DESC").
		Find(&costs).Error; err != nil {
		return nil, err
	}

	matches := make([]ChannelModelCost, 0, len(costs))
	seen := make(map[int]struct{}, len(costs))
	for _, cost := range costs {
		channel, ok := channelByID[cost.ChannelId]
		if !ok {
			continue
		}
		upstreamModel := upstreamModelForChannelCost(channel, publicModel)
		if cost.UpstreamModel == publicModel || cost.UpstreamModel == upstreamModel {
			if _, ok := seen[cost.Id]; ok {
				continue
			}
			seen[cost.Id] = struct{}{}
			matches = append(matches, cost)
		}
	}
	if len(matches) == 0 {
		return nil, ErrMoneyPricingPolicyNotFound
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].ChannelId == matches[j].ChannelId {
			return matches[i].Id > matches[j].Id
		}
		return matches[i].ChannelId < matches[j].ChannelId
	})
	return matches, nil
}

func upstreamModelForChannelCost(channel Channel, publicModel string) string {
	upstream := strings.TrimSpace(publicModel)
	modelMapping := channel.GetModelMapping()
	if strings.TrimSpace(modelMapping) == "" || strings.TrimSpace(modelMapping) == "{}" {
		return upstream
	}
	modelMap := make(map[string]string)
	if err := common.UnmarshalJsonStr(modelMapping, &modelMap); err != nil {
		return upstream
	}
	current := upstream
	visited := map[string]struct{}{current: {}}
	for {
		next := strings.TrimSpace(modelMap[current])
		if next == "" {
			return current
		}
		if _, ok := visited[next]; ok {
			return current
		}
		visited[next] = struct{}{}
		current = next
	}
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

func ListEnabledRetailPricingPolicies(groups []string) ([]RetailPricingPolicy, error) {
	var policies []RetailPricingPolicy
	query := DB.Where("enabled = ?", true).Order("id ASC")
	cleanGroups := make([]string, 0, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group != "" {
			cleanGroups = append(cleanGroups, group)
		}
	}
	if len(cleanGroups) > 0 {
		query = query.Where(fmt.Sprintf("%s IN ?", retailPricingGroupColumn()), cleanGroups)
	}
	if err := query.Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func ListChannelModelCosts(offset int, limit int) ([]ChannelModelCost, int64, error) {
	var costs []ChannelModelCost
	var total int64
	query := DB.Model(&ChannelModelCost{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = DB.Order("id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	if err := query.Find(&costs).Error; err != nil {
		return nil, 0, err
	}
	return costs, total, nil
}

func GetChannelModelCostById(id int) (*ChannelModelCost, error) {
	var cost ChannelModelCost
	err := DB.First(&cost, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMoneyPricingPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cost, nil
}

func DeleteChannelModelCostById(id int) error {
	result := DB.Delete(&ChannelModelCost{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMoneyPricingPolicyNotFound
	}
	return nil
}

func ListRetailPricingPolicies(offset int, limit int) ([]RetailPricingPolicy, int64, error) {
	var policies []RetailPricingPolicy
	var total int64
	query := DB.Model(&RetailPricingPolicy{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = DB.Order("id DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	if err := query.Find(&policies).Error; err != nil {
		return nil, 0, err
	}
	return policies, total, nil
}

func GetRetailPricingPolicyById(id int) (*RetailPricingPolicy, error) {
	var policy RetailPricingPolicy
	err := DB.First(&policy, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMoneyPricingPolicyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func DeleteRetailPricingPolicyById(id int) error {
	result := DB.Delete(&RetailPricingPolicy{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrMoneyPricingPolicyNotFound
	}
	return nil
}

func BuildRetailPricingPolicyLookup(policies []RetailPricingPolicy) RetailPricingPolicyLookup {
	lookup := make(RetailPricingPolicyLookup, len(policies))
	for i := range policies {
		policy := &policies[i]
		lookup[retailPricingPolicyLookupKey(policy.PublicModel, policy.Group, policy.EndpointType)] = policy
	}
	return lookup
}

func (lookup RetailPricingPolicyLookup) Lookup(publicModel string, group string, endpointType string) (*RetailPricingPolicy, bool) {
	group = strings.TrimSpace(group)
	if group == "" {
		group = DefaultPricingGroup
	}
	if policy, ok := lookup[retailPricingPolicyLookupKey(publicModel, group, endpointType)]; ok {
		return policy, true
	}
	if group != DefaultPricingGroup {
		if policy, ok := lookup[retailPricingPolicyLookupKey(publicModel, DefaultPricingGroup, endpointType)]; ok {
			return policy, true
		}
	}
	return nil, false
}

func retailPricingPolicyLookupKey(publicModel string, group string, endpointType string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		group = DefaultPricingGroup
	}
	return strings.TrimSpace(publicModel) + "\x00" + group + "\x00" + strings.TrimSpace(endpointType)
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
