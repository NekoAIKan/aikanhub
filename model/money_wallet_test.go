package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMoneyWalletTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&MoneyWallet{}, &MoneyWalletTransaction{}))
	require.NoError(t, DB.Exec("DELETE FROM money_wallet_transactions").Error)
	require.NoError(t, DB.Exec("DELETE FROM money_wallets").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM money_wallet_transactions")
		DB.Exec("DELETE FROM money_wallets")
	})
}

func getMoneyWalletForTest(t *testing.T, userID int, currency string) MoneyWallet {
	t.Helper()
	var wallet MoneyWallet
	require.NoError(t, DB.Where("user_id = ? AND currency = ?", userID, currency).First(&wallet).Error)
	return wallet
}

func countMoneyWalletTransactionsForTest(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&MoneyWalletTransaction{}).Count(&count).Error)
	return count
}

func TestMoneyWalletCreditFreezeSettleAndReleaseDifference(t *testing.T) {
	setupMoneyWalletTest(t)
	require.NoError(t, CreditWallet(1, "usd", 2_000_000, "topup-1", MoneyWalletTransactionTopup, ""))
	require.NoError(t, FreezeWallet(1, "USD", 1_500_000, "req-1", ""))
	require.NoError(t, SettleFrozenWallet(1, "USD", "req-1", 1_200_000, "settle-1", ""))

	wallet := getMoneyWalletForTest(t, 1, "USD")
	assert.Equal(t, int64(800_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	assert.Equal(t, int64(2_000_000), wallet.LifetimeTopupMicros)
	assert.Equal(t, int64(3), countMoneyWalletTransactionsForTest(t))
}

func TestMoneyWalletSettleCanChargeMoreThanFrozenWhenAvailable(t *testing.T) {
	setupMoneyWalletTest(t)
	require.NoError(t, CreditWallet(1, "USD", 2_000_000, "topup-1", MoneyWalletTransactionTopup, ""))
	require.NoError(t, FreezeWallet(1, "USD", 1_000_000, "req-1", ""))
	require.NoError(t, SettleFrozenWallet(1, "USD", "req-1", 1_250_000, "settle-1", ""))

	wallet := getMoneyWalletForTest(t, 1, "USD")
	assert.Equal(t, int64(750_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
}

func TestMoneyWalletPreauthCannotBeReleasedAfterSettle(t *testing.T) {
	setupMoneyWalletTest(t)
	require.NoError(t, CreditWallet(1, "USD", 2_000_000, "topup-1", MoneyWalletTransactionTopup, ""))
	require.NoError(t, FreezeWallet(1, "USD", 500_000, "req-a", ""))
	require.NoError(t, FreezeWallet(1, "USD", 500_000, "req-b", ""))
	require.NoError(t, SettleFrozenWallet(1, "USD", "req-a", 500_000, "settle-a", ""))

	err := ReleaseFrozenWallet(1, "USD", "req-a", "release-a", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMoneyWalletPreauthClosed))

	wallet := getMoneyWalletForTest(t, 1, "USD")
	assert.Equal(t, int64(1_000_000), wallet.AvailableMicros)
	assert.Equal(t, int64(500_000), wallet.FrozenMicros)
}

func TestMoneyWalletReleaseFrozen(t *testing.T) {
	setupMoneyWalletTest(t)
	require.NoError(t, CreditWallet(1, "USD", 1_000_000, "topup-1", MoneyWalletTransactionTopup, ""))
	require.NoError(t, FreezeWallet(1, "USD", 750_000, "req-1", ""))
	require.NoError(t, ReleaseFrozenWallet(1, "USD", "req-1", "release-1", ""))

	wallet := getMoneyWalletForTest(t, 1, "USD")
	assert.Equal(t, int64(1_000_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
}

func TestMoneyWalletDuplicateRequestIsIdempotent(t *testing.T) {
	setupMoneyWalletTest(t)
	require.NoError(t, CreditWallet(1, "USD", 1_000_000, "topup-1", MoneyWalletTransactionTopup, ""))
	require.NoError(t, CreditWallet(1, "USD", 1_000_000, "topup-1", MoneyWalletTransactionTopup, ""))

	wallet := getMoneyWalletForTest(t, 1, "USD")
	assert.Equal(t, int64(1_000_000), wallet.AvailableMicros)
	assert.Equal(t, int64(1_000_000), wallet.LifetimeTopupMicros)
	assert.Equal(t, int64(1), countMoneyWalletTransactionsForTest(t))
}

func TestMoneyWalletInsufficientFundsLeavesWalletUnchanged(t *testing.T) {
	setupMoneyWalletTest(t)
	require.NoError(t, CreditWallet(1, "USD", 500_000, "topup-1", MoneyWalletTransactionTopup, ""))

	err := FreezeWallet(1, "USD", 750_000, "req-1", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMoneyWalletInsufficientFunds))

	wallet := getMoneyWalletForTest(t, 1, "USD")
	assert.Equal(t, int64(500_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	assert.Equal(t, int64(1), countMoneyWalletTransactionsForTest(t))
}

func TestMoneyWalletAdjustmentAndRefund(t *testing.T) {
	setupMoneyWalletTest(t)
	require.NoError(t, AdjustWallet(1, "USD", 500_000, "adjust-1", ""))
	require.NoError(t, RefundWallet(1, "USD", 250_000, "refund-1", ""))

	wallet := getMoneyWalletForTest(t, 1, "USD")
	assert.Equal(t, int64(750_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.LifetimeTopupMicros)
}

func TestRechargeCreditsMoneyWallet(t *testing.T) {
	resetMoneyBillingModeForWalletTest(t, "legacy")
	truncateTables(t)
	setupMoneyWalletTest(t)
	insertUserForPaymentGuardTest(t, 501, 0)
	insertTopUpForPaymentGuardTest(t, "stripe-money-wallet", 501, PaymentProviderStripe)

	err := Recharge("stripe-money-wallet", "cus_test", "127.0.0.1")
	require.NoError(t, err)

	wallet := getMoneyWalletForTest(t, 501, "USD")
	assert.Equal(t, int64(9_990_000), wallet.AvailableMicros)
	assert.Equal(t, int64(9_990_000), wallet.LifetimeTopupMicros)
	assert.Equal(t, int64(1), countMoneyWalletTransactionsForTest(t))
}

func TestRechargeWaffoPancakeCreditsMoneyWalletOnce(t *testing.T) {
	resetMoneyBillingModeForWalletTest(t, "legacy")
	truncateTables(t)
	setupMoneyWalletTest(t)
	insertUserForPaymentGuardTest(t, 502, 0)
	insertTopUpForPaymentGuardTest(t, "waffo-pancake-money-wallet", 502, PaymentProviderWaffoPancake)

	require.NoError(t, RechargeWaffoPancake("waffo-pancake-money-wallet"))
	require.NoError(t, RechargeWaffoPancake("waffo-pancake-money-wallet"))

	wallet := getMoneyWalletForTest(t, 502, "USD")
	assert.Equal(t, int64(9_990_000), wallet.AvailableMicros)
	assert.Equal(t, int64(9_990_000), wallet.LifetimeTopupMicros)
	assert.Equal(t, int64(1), countMoneyWalletTransactionsForTest(t))
}

func TestRechargeMoneyModeDoesNotMirrorQuota(t *testing.T) {
	resetMoneyBillingModeForWalletTest(t, "money")
	truncateTables(t)
	setupMoneyWalletTest(t)
	insertUserForPaymentGuardTest(t, 503, 0)
	insertTopUpForPaymentGuardTest(t, "stripe-money-mode-wallet", 503, PaymentProviderStripe)

	err := Recharge("stripe-money-mode-wallet", "cus_test", "127.0.0.1")
	require.NoError(t, err)

	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 503))
	wallet := getMoneyWalletForTest(t, 503, "USD")
	assert.Equal(t, int64(9_990_000), wallet.AvailableMicros)
}

func TestRechargeWaffoAndCreemCreditMoneyWallet(t *testing.T) {
	resetMoneyBillingModeForWalletTest(t, "legacy")
	truncateTables(t)
	setupMoneyWalletTest(t)
	insertUserForPaymentGuardTest(t, 504, 0)
	insertTopUpForPaymentGuardTest(t, "waffo-money-wallet", 504, PaymentProviderWaffo)
	insertTopUpForPaymentGuardTest(t, "creem-money-wallet", 504, PaymentProviderCreem)

	require.NoError(t, RechargeWaffo("waffo-money-wallet", "127.0.0.1"))
	require.NoError(t, RechargeCreem("creem-money-wallet", "", "", "127.0.0.1"))

	assert.Equal(t, int64(19_980_000), getMoneyWalletForTest(t, 504, "USD").AvailableMicros)
}

func resetMoneyBillingModeForWalletTest(t *testing.T, mode string) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  mode,
		"billing_setting.settlement_currency": "USD",
	}))
	t.Cleanup(func() {
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
		})
	})
}
