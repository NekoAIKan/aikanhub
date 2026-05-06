package credit_display_setting

import "github.com/QuantumNous/new-api/setting/config"

type CreditDisplaySetting struct {
	Enabled        bool   `json:"enabled"`
	Label          string `json:"label"`
	QuotaPerCredit int    `json:"quota_per_credit"`
	Precision      int    `json:"precision"`
}

var creditDisplaySetting = defaultCreditDisplaySetting()

func init() {
	config.GlobalConfig.Register("credit_display_setting", &creditDisplaySetting)
}

func defaultCreditDisplaySetting() CreditDisplaySetting {
	return CreditDisplaySetting{
		Enabled:        true,
		Label:          "Credits",
		QuotaPerCredit: 5000,
		Precision:      2,
	}
}

func GetCreditDisplaySetting() CreditDisplaySetting {
	setting := creditDisplaySetting
	defaults := defaultCreditDisplaySetting()
	if setting.Label == "" {
		setting.Label = defaults.Label
	}
	if setting.QuotaPerCredit <= 0 {
		setting.QuotaPerCredit = defaults.QuotaPerCredit
	}
	if setting.Precision < 0 {
		setting.Precision = defaults.Precision
	}
	return setting
}
