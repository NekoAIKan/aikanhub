package billing_visibility_setting

import "github.com/QuantumNous/new-api/setting/config"

type BillingVisibilitySetting struct {
	DefaultMode string            `json:"default_mode"`
	GroupModes  map[string]string `json:"group_modes"`
}

var billingVisibilitySetting = defaultBillingVisibilitySetting()

func init() {
	config.GlobalConfig.Register("billing_visibility_setting", &billingVisibilitySetting)
}

func defaultBillingVisibilitySetting() BillingVisibilitySetting {
	return BillingVisibilitySetting{
		DefaultMode: "credits",
		GroupModes: map[string]string{
			"default":    "credits",
			"trial":      "credits",
			"beta":       "summary",
			"invited":    "detailed",
			"b2b":        "detailed",
			"enterprise": "detailed",
		},
	}
}

func GetBillingVisibilitySetting() BillingVisibilitySetting {
	setting := billingVisibilitySetting
	defaults := defaultBillingVisibilitySetting()
	if setting.DefaultMode == "" {
		setting.DefaultMode = defaults.DefaultMode
	}
	groupModes := make(map[string]string, len(defaults.GroupModes)+len(setting.GroupModes))
	for k, v := range defaults.GroupModes {
		groupModes[k] = v
	}
	for k, v := range setting.GroupModes {
		groupModes[k] = v
	}
	setting.GroupModes = groupModes
	return setting
}
