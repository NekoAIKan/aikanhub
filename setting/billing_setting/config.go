package billing_setting

const (
	MoneyBillingModeLegacy   = "legacy"
	MoneyBillingModeDualRead = "dual_read"
	MoneyBillingModeMoney    = "money"
)

type MoneyBillingSettingView struct {
	MoneyBillingMode   string
	SettlementCurrency string
}

func defaultMoneyBillingSetting() MoneyBillingSettingView {
	return MoneyBillingSettingView{
		MoneyBillingMode:   MoneyBillingModeLegacy,
		SettlementCurrency: "USD",
	}
}

func GetMoneyBillingSetting() MoneyBillingSettingView {
	defaults := defaultMoneyBillingSetting()
	setting := MoneyBillingSettingView{
		MoneyBillingMode:   billingSetting.MoneyBillingMode,
		SettlementCurrency: billingSetting.SettlementCurrency,
	}
	if !IsValidMoneyBillingMode(setting.MoneyBillingMode) {
		setting.MoneyBillingMode = defaults.MoneyBillingMode
	}
	if setting.SettlementCurrency == "" {
		setting.SettlementCurrency = defaults.SettlementCurrency
	}
	return setting
}

func GetMoneyBillingMode() string {
	return GetMoneyBillingSetting().MoneyBillingMode
}

func IsMoneyBillingModeEnabled() bool {
	return GetMoneyBillingMode() == MoneyBillingModeMoney
}

func IsValidMoneyBillingMode(mode string) bool {
	switch mode {
	case MoneyBillingModeLegacy, MoneyBillingModeDualRead, MoneyBillingModeMoney:
		return true
	default:
		return false
	}
}
