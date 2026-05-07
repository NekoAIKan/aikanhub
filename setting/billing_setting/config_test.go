package billing_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestMoneyBillingSettingDefaults(t *testing.T) {
	resetMoneyBillingSetting(t)

	got := GetMoneyBillingSetting()
	require.Equal(t, MoneyBillingModeLegacy, got.MoneyBillingMode)
	require.Equal(t, "USD", got.SettlementCurrency)
}

func TestMoneyBillingSettingLoadsFromConfig(t *testing.T) {
	resetMoneyBillingSetting(t)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "money",
		"billing_setting.settlement_currency": "CNY",
	}))

	got := GetMoneyBillingSetting()
	require.Equal(t, MoneyBillingModeMoney, got.MoneyBillingMode)
	require.Equal(t, "CNY", got.SettlementCurrency)
	require.True(t, IsMoneyBillingModeEnabled())
}

func TestMoneyBillingSettingFallsBackForInvalidMode(t *testing.T) {
	resetMoneyBillingSetting(t)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode": "invalid",
	}))

	require.Equal(t, MoneyBillingModeLegacy, GetMoneyBillingMode())
}

func resetMoneyBillingSetting(t *testing.T) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "legacy",
		"billing_setting.settlement_currency": "USD",
	}))
}
