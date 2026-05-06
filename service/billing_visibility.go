package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_visibility_setting"
)

const (
	BillingVisibilityCredits  = "credits"
	BillingVisibilitySummary  = "summary"
	BillingVisibilityDetailed = "detailed"
	BillingVisibilityInternal = "internal"
)

func GetBillingVisibilityMode(role int, group string) string {
	if role >= common.RoleAdminUser {
		return BillingVisibilityInternal
	}

	setting := billing_visibility_setting.GetBillingVisibilitySetting()
	group = strings.TrimSpace(group)
	if group == "" {
		group = "default"
	}
	if mode, ok := setting.GroupModes[group]; ok {
		return userBillingVisibilityMode(normalizeBillingVisibilityMode(mode, setting.DefaultMode))
	}
	return userBillingVisibilityMode(normalizeBillingVisibilityMode(setting.DefaultMode, BillingVisibilityCredits))
}

func normalizeBillingVisibilityMode(mode string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case BillingVisibilityCredits:
		return BillingVisibilityCredits
	case BillingVisibilitySummary:
		return BillingVisibilitySummary
	case BillingVisibilityDetailed:
		return BillingVisibilityDetailed
	case BillingVisibilityInternal:
		return BillingVisibilityInternal
	}
	if fallback == "" || strings.EqualFold(fallback, mode) {
		return BillingVisibilityCredits
	}
	return normalizeBillingVisibilityMode(fallback, BillingVisibilityCredits)
}

func userBillingVisibilityMode(mode string) string {
	if mode == BillingVisibilityInternal {
		return BillingVisibilityDetailed
	}
	return mode
}
