package service

import (
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/credit_display_setting"
)

func BuildTaskBillingBreakdown(task *model.Task, mode string) map[string]any {
	if task == nil {
		return nil
	}

	record := billingBreakdownRecord{
		ChargedQuota: task.Quota,
		ActualQuota:  task.Quota,
		GroupRatio:   1,
	}
	if task.Properties.OriginModelName != "" {
		record.Model = task.Properties.OriginModelName
	} else {
		record.Model = task.Properties.UpstreamModelName
	}

	if bc := task.PrivateData.BillingContext; bc != nil {
		record.Model = firstNonEmpty(bc.OriginModelName, bc.BillingProfile, record.Model)
		record.Basis = bc.BillingBasis
		record.Tokens = bc.EstimatedTokens
		record.PrechargedQuota = bc.EstimatedQuota
		record.GroupRatio = bc.GroupRatio
		record.RetailUnitPrice = bc.RetailUnitPrice
		record.UpstreamUnitCost = bc.UpstreamUnitCost
		record.MarkupPercent = bc.MarkupPercent
		record.PricingVersion = bc.PricingVersion
		record.PricingHash = bc.PricingHash
		record.VideoParams = bc.VideoParams
	}
	return buildBillingBreakdown(record, mode)
}

func BuildLogBillingBreakdown(log *model.Log, mode string) map[string]any {
	if log == nil {
		return nil
	}
	other, _ := common.StrToMap(log.Other)
	if other == nil {
		other = map[string]any{}
	}

	actualQuota := intFromAny(other["actual_quota"])
	if actualQuota <= 0 {
		actualQuota = intFromAny(other["charged_quota"])
	}
	if actualQuota <= 0 {
		actualQuota = log.Quota
	}

	prechargedQuota := intFromAny(other["pre_consumed_quota"])
	if prechargedQuota <= 0 {
		prechargedQuota = intFromAny(other["estimated_quota"])
	}

	record := billingBreakdownRecord{
		ChargedQuota:     actualQuota,
		ActualQuota:      actualQuota,
		PrechargedQuota:  prechargedQuota,
		Model:            firstNonEmpty(stringFromAny(other["origin_model_name"]), stringFromAny(other["billing_profile"]), log.ModelName),
		Basis:            stringFromAny(other["billing_basis"]),
		Tokens:           intFromAny(other["actual_tokens"]),
		GroupRatio:       floatFromAny(other["group_ratio"]),
		RetailUnitPrice:  floatFromAny(other["retail_unit_price"]),
		UpstreamUnitCost: floatFromAny(other["upstream_unit_cost"]),
		MarkupPercent:    floatFromAny(other["markup_percent"]),
		PricingVersion:   stringFromAny(other["pricing_version"]),
		PricingHash:      stringFromAny(other["pricing_hash"]),
		VideoParams:      mapFromAny(other["video_params"]),
	}
	if record.Tokens <= 0 {
		record.Tokens = intFromAny(other["estimated_tokens"])
	}
	if record.GroupRatio <= 0 {
		record.GroupRatio = 1
	}
	if record.RetailUnitPrice <= 0 && record.Tokens > 0 && record.ActualQuota > 0 && record.GroupRatio > 0 {
		record.RetailUnitPrice = float64(record.ActualQuota) / common.QuotaPerUnit * 1_000_000 / float64(record.Tokens) / record.GroupRatio
	}
	return buildBillingBreakdown(record, mode)
}

func SanitizeLogOtherForBillingVisibility(log *model.Log, mode string) {
	if log == nil || log.Other == "" {
		return
	}
	other, _ := common.StrToMap(log.Other)
	if other == nil {
		return
	}
	mode = normalizeBillingVisibilityMode(mode, BillingVisibilityCredits)
	deleteAdminBillingFields(other)
	deleteVideoBillingAliasFields(other)
	if mode == BillingVisibilityCredits {
		deleteFormulaBillingFields(other)
	}
	log.Other = common.MapToJsonStr(other)
}

type billingBreakdownRecord struct {
	ChargedQuota     int
	ActualQuota      int
	PrechargedQuota  int
	Model            string
	Basis            string
	Tokens           int
	GroupRatio       float64
	RetailUnitPrice  float64
	UpstreamUnitCost float64
	MarkupPercent    float64
	PricingVersion   string
	PricingHash      string
	VideoParams      map[string]any
}

func buildBillingBreakdown(record billingBreakdownRecord, mode string) map[string]any {
	mode = normalizeBillingVisibilityMode(mode, BillingVisibilityCredits)
	creditConfig := credit_display_setting.GetCreditDisplaySetting()
	breakdown := map[string]any{
		"mode":            mode,
		"charged_credits": quotaToCredits(record.ChargedQuota, creditConfig),
		"label":           creditConfig.Label,
	}

	if mode == BillingVisibilityCredits {
		return breakdown
	}

	if record.Model != "" {
		breakdown["model"] = record.Model
	}
	if record.Basis != "" {
		breakdown["basis"] = record.Basis
	}
	addVideoSummaryFields(breakdown, record.VideoParams)

	if record.PrechargedQuota > 0 && mode == BillingVisibilitySummary {
		breakdown["adjustment_credits"] = quotaToCredits(record.ActualQuota-record.PrechargedQuota, creditConfig)
	}

	if mode == BillingVisibilitySummary {
		return breakdown
	}

	breakdown["charged_quota"] = record.ChargedQuota
	if record.Tokens > 0 {
		breakdown["tokens"] = record.Tokens
	}
	if record.RetailUnitPrice > 0 {
		breakdown["unit_price_per_million"] = record.RetailUnitPrice
	}
	if record.GroupRatio > 0 {
		breakdown["group_ratio"] = record.GroupRatio
	}
	if record.PrechargedQuota > 0 {
		breakdown["precharged_quota"] = record.PrechargedQuota
		breakdown["actual_quota"] = record.ActualQuota
		breakdown["adjustment_quota"] = record.ActualQuota - record.PrechargedQuota
	}
	if record.PricingVersion != "" {
		breakdown["pricing_version"] = record.PricingVersion
	}
	if record.PricingHash != "" {
		breakdown["pricing_hash"] = record.PricingHash
	}

	if mode != BillingVisibilityInternal {
		return breakdown
	}

	if record.UpstreamUnitCost > 0 {
		upstreamCostQuota := upstreamCostQuota(record.Tokens, record.UpstreamUnitCost)
		breakdown["upstream_unit_cost_per_million"] = record.UpstreamUnitCost
		breakdown["upstream_cost_quota"] = upstreamCostQuota
		grossMarginQuota := record.ActualQuota - upstreamCostQuota
		breakdown["gross_margin_quota"] = grossMarginQuota
		if record.ActualQuota > 0 {
			breakdown["gross_margin_percent"] = roundFloat(float64(grossMarginQuota)/float64(record.ActualQuota)*100, 2)
		}
	}
	if record.MarkupPercent != 0 {
		breakdown["markup_percent"] = record.MarkupPercent
	}
	return breakdown
}

func addVideoSummaryFields(breakdown map[string]any, params map[string]any) {
	if len(params) == 0 {
		return
	}
	if outputSeconds := intFromAny(params["output_seconds"]); outputSeconds > 0 {
		breakdown["output_seconds"] = outputSeconds
	}
	if inputSeconds := intFromAny(params["input_seconds"]); inputSeconds > 0 {
		breakdown["input_seconds"] = inputSeconds
	}
	width := intFromAny(params["width"])
	height := intFromAny(params["height"])
	if width > 0 && height > 0 {
		breakdown["resolution"] = strconv.Itoa(width) + "x" + strconv.Itoa(height)
	}
	if fps := intFromAny(params["fps"]); fps > 0 {
		breakdown["fps"] = fps
	}
	if boolFromAny(params["draft"]) {
		breakdown["draft"] = true
	}
}

func quotaToCredits(quota int, config credit_display_setting.CreditDisplaySetting) float64 {
	if config.QuotaPerCredit <= 0 {
		config.QuotaPerCredit = 5000
	}
	return roundFloat(float64(quota)/float64(config.QuotaPerCredit), config.Precision)
}

func upstreamCostQuota(tokens int, upstreamUnitCost float64) int {
	if tokens <= 0 || upstreamUnitCost <= 0 {
		return 0
	}
	return int(float64(tokens) * upstreamUnitCost / 1_000_000 * common.QuotaPerUnit)
}

func roundFloat(value float64, precision int) float64 {
	if precision < 0 {
		precision = 2
	}
	multiplier := math.Pow10(precision)
	return math.Round(value*multiplier) / multiplier
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func floatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case string:
		n, _ := strconv.ParseFloat(v, 64)
		return n
	default:
		return 0
	}
}

func boolFromAny(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, _ := strconv.ParseBool(v)
		return parsed
	default:
		return false
	}
}

func stringFromAny(value any) string {
	v, _ := value.(string)
	return v
}

func mapFromAny(value any) map[string]any {
	v, _ := value.(map[string]any)
	return v
}

func deleteAdminBillingFields(other map[string]interface{}) {
	delete(other, "retail_unit_price")
	delete(other, "upstream_unit_cost")
	delete(other, "upstream_unit_cost_per_million")
	delete(other, "upstream_cost_quota")
	delete(other, "markup_percent")
	delete(other, "gross_margin_quota")
	delete(other, "gross_margin_percent")
	delete(other, "pricing_hash")
}

func deleteVideoBillingAliasFields(other map[string]interface{}) {
	for key := range other {
		if strings.HasPrefix(key, "video_") {
			delete(other, key)
		}
	}
}

func deleteFormulaBillingFields(other map[string]interface{}) {
	delete(other, "model_price")
	delete(other, "model_ratio")
	delete(other, "group_ratio")
	delete(other, "user_group_ratio")
	delete(other, "billing_mode")
	delete(other, "billing_profile")
	delete(other, "billing_basis")
	delete(other, "origin_model_name")
	delete(other, "estimated_tokens")
	delete(other, "estimated_quota")
	delete(other, "pre_consumed_quota")
	delete(other, "actual_quota")
	delete(other, "actual_tokens")
	delete(other, "video_params")
	delete(other, "has_reference_media")
	delete(other, "pricing_version")
}
