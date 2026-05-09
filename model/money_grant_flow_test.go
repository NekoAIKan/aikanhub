package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMoneyGrantFlowTest(t *testing.T) {
	t.Helper()
	ensureModelTestSchema(t, &User{}, &Token{}, &Log{}, &MoneyWallet{}, &MoneyWalletTransaction{}, &Redemption{}, &Checkin{})
	for _, table := range []string{
		"money_wallet_transactions",
		"money_wallets",
		"redemptions",
		"checkins",
		"tokens",
		"logs",
		"users",
	} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "legacy",
		"billing_setting.settlement_currency": "USD",
		"checkin_setting.enabled":             "false",
		"onboarding_setting.new_user_quota":   "0",
	}))
	t.Cleanup(func() {
		for _, table := range []string{
			"money_wallet_transactions",
			"money_wallets",
			"redemptions",
			"checkins",
			"tokens",
			"logs",
			"users",
		} {
			DB.Exec("DELETE FROM " + table)
		}
		common.QuotaPerUnit = originalQuotaPerUnit
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
			"checkin_setting.enabled":             "false",
			"onboarding_setting.new_user_quota":   "0",
		})
	})
}

func setMoneyModeForGrantFlowTest(t *testing.T) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "money",
		"billing_setting.settlement_currency": "USD",
	}))
}

func fetchGrantFlowUser(t *testing.T, id int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, "id = ?", id).Error)
	return user
}

func fetchGrantFlowWallet(t *testing.T, userID int) MoneyWallet {
	t.Helper()
	var wallet MoneyWallet
	require.NoError(t, DB.First(&wallet, "user_id = ? AND currency = ?", userID, "USD").Error)
	return wallet
}

func TestGrantUserQuotaOrMoneyWritesWalletOnlyInMoneyMode(t *testing.T) {
	setupMoneyGrantFlowTest(t)
	setMoneyModeForGrantFlowTest(t)
	require.NoError(t, DB.Create(&User{Id: 901, Username: "grant-money", AffCode: "gm01", Status: common.UserStatusEnabled}).Error)

	require.NoError(t, GrantUserQuotaOrMoney(901, 500_000, "grant:user:901", "test_grant"))

	assert.Equal(t, 0, fetchGrantFlowUser(t, 901).Quota)
	wallet := fetchGrantFlowWallet(t, 901)
	assert.Equal(t, int64(1_000_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.LifetimeTopupMicros)

	var transaction MoneyWalletTransaction
	require.NoError(t, DB.First(&transaction, "request_id = ?", "grant:user:901").Error)
	assert.Equal(t, MoneyWalletTransactionGrant, transaction.Type)
}

func TestRedeemCreditsMoneyWalletInMoneyMode(t *testing.T) {
	setupMoneyGrantFlowTest(t)
	setMoneyModeForGrantFlowTest(t)
	require.NoError(t, DB.Create(&User{Id: 902, Username: "redeem-money", AffCode: "rm01", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&Redemption{
		Key:    "redeem-money-mode-key",
		Status: common.RedemptionCodeStatusEnabled,
		Name:   "money-mode-redemption",
		Quota:  500_000,
	}).Error)

	quota, err := Redeem("redeem-money-mode-key", 902)
	require.NoError(t, err)

	assert.Equal(t, 500_000, quota)
	assert.Equal(t, 0, fetchGrantFlowUser(t, 902).Quota)
	assert.Equal(t, int64(1_000_000), fetchGrantFlowWallet(t, 902).AvailableMicros)

	var redemption Redemption
	require.NoError(t, DB.First(&redemption, commonKeyCol+" = ?", "redeem-money-mode-key").Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, redemption.Status)
	assert.Equal(t, 902, redemption.UsedUserId)
}

func TestCheckinCreditsMoneyWalletInMoneyMode(t *testing.T) {
	setupMoneyGrantFlowTest(t)
	setMoneyModeForGrantFlowTest(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"checkin_setting.enabled":   "true",
		"checkin_setting.min_quota": "500000",
		"checkin_setting.max_quota": "500000",
	}))
	require.NoError(t, DB.Create(&User{Id: 903, Username: "checkin-money", AffCode: "cm01", Status: common.UserStatusEnabled}).Error)

	checkin, err := UserCheckin(903)
	require.NoError(t, err)

	assert.Equal(t, 500_000, checkin.QuotaAwarded)
	assert.Equal(t, 0, fetchGrantFlowUser(t, 903).Quota)
	assert.Equal(t, int64(1_000_000), fetchGrantFlowWallet(t, 903).AvailableMicros)
}

func TestTransferAffQuotaCreditsMoneyWalletInMoneyMode(t *testing.T) {
	setupMoneyGrantFlowTest(t)
	setMoneyModeForGrantFlowTest(t)
	require.NoError(t, DB.Create(&User{Id: 904, Username: "aff-money", AffCode: "am01", AffQuota: 500_000, Status: common.UserStatusEnabled}).Error)
	user := fetchGrantFlowUser(t, 904)

	require.NoError(t, user.TransferAffQuotaToQuota(500_000))

	updated := fetchGrantFlowUser(t, 904)
	assert.Equal(t, 0, updated.Quota)
	assert.Equal(t, 0, updated.AffQuota)
	assert.Equal(t, int64(1_000_000), fetchGrantFlowWallet(t, 904).AvailableMicros)
}

func TestUserInsertOnboardingCreditsMoneyWalletInMoneyMode(t *testing.T) {
	setupMoneyGrantFlowTest(t)
	setMoneyModeForGrantFlowTest(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"onboarding_setting.new_user_quota":          "500000",
		"onboarding_setting.default_token_enabled":   "0",
		"onboarding_setting.default_token_quota":     "0",
		"onboarding_setting.default_token_unlimited": "false",
	}))

	user := &User{
		Username: "onboarding-money",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, user.Insert(0))

	created := fetchGrantFlowUser(t, user.Id)
	assert.Equal(t, 0, created.Quota)
	assert.Equal(t, int64(1_000_000), fetchGrantFlowWallet(t, user.Id).AvailableMicros)
}

func TestAdminLegacyQuotaWalletAdjustAndOverrideInMoneyMode(t *testing.T) {
	setupMoneyGrantFlowTest(t)
	setMoneyModeForGrantFlowTest(t)
	require.NoError(t, DB.Create(&User{Id: 905, Username: "admin-money", AffCode: "adm1", Status: common.UserStatusEnabled}).Error)

	require.NoError(t, AdjustWalletLegacyQuota(905, 500_000, "admin:add:905", "admin_add"))
	assert.Equal(t, int64(1_000_000), fetchGrantFlowWallet(t, 905).AvailableMicros)

	require.NoError(t, AdjustWalletLegacyQuota(905, -250_000, "admin:subtract:905", "admin_subtract"))
	assert.Equal(t, int64(500_000), fetchGrantFlowWallet(t, 905).AvailableMicros)

	require.NoError(t, OverrideWalletLegacyQuota(905, 1_000_000, "admin:override:905", "admin_override"))
	assert.Equal(t, int64(2_000_000), fetchGrantFlowWallet(t, 905).AvailableMicros)
	assert.Equal(t, 0, fetchGrantFlowUser(t, 905).Quota)
}
