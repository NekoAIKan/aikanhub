package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupQuotaMoneyBackfillTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}, &model.UserSubscription{}, &model.MoneyWallet{}, &model.MoneyWalletTransaction{}))
	require.NoError(t, model.DB.Exec("DELETE FROM money_wallet_transactions").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM money_wallets").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM user_subscriptions").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM tokens").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM users").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM money_wallet_transactions")
		model.DB.Exec("DELETE FROM money_wallets")
		model.DB.Exec("DELETE FROM user_subscriptions")
		model.DB.Exec("DELETE FROM tokens")
		model.DB.Exec("DELETE FROM users")
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
		})
	})
}

func TestQuotaMoneyBackfillPreviewReportsUsersTokensAndSubscriptions(t *testing.T) {
	setupQuotaMoneyBackfillTest(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	require.NoError(t, model.DB.Create(&model.User{Id: 701, Username: "u701", AffCode: "u701", Quota: 500000, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.DB.Create(&model.Token{UserId: 701, Key: "token701", RemainQuota: 250000}).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{UserId: 701, AmountTotal: 1000000, AmountUsed: 250000, Status: "active"}).Error)

	preview, err := PreviewQuotaMoneyBackfill("usd")
	require.NoError(t, err)

	assert.Equal(t, "USD", preview.Currency)
	assert.Equal(t, int64(1), preview.UserCount)
	assert.Equal(t, int64(1), preview.TokenCount)
	assert.Equal(t, int64(1), preview.SubscriptionCount)
	assert.Equal(t, int64(500000), preview.UserQuotaTotal)
	assert.Equal(t, int64(250000), preview.TokenRemainQuotaTotal)
	assert.Equal(t, int64(750000), preview.SubscriptionRemainingQuotaTotal)
	assert.Equal(t, int64(1_000_000), preview.UserAmountMicrosTotal)
	assert.Equal(t, int64(500_000), preview.TokenRemainAmountMicrosTotal)
	assert.Equal(t, int64(1_500_000), preview.SubscriptionAmountMicrosTotal)
}

func TestQuotaMoneyBackfillApplyIsIdempotent(t *testing.T) {
	setupQuotaMoneyBackfillTest(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	require.NoError(t, model.DB.Create(&model.User{Id: 702, Username: "u702", AffCode: "u702", Quota: 1_000_000, Status: common.UserStatusEnabled}).Error)

	first, err := ApplyQuotaMoneyBackfill("USD")
	require.NoError(t, err)
	second, err := ApplyQuotaMoneyBackfill("USD")
	require.NoError(t, err)

	assert.Equal(t, int64(1), first.UserTransactionsSeen)
	assert.Equal(t, int64(1), second.UserTransactionsSeen)

	var wallet model.MoneyWallet
	require.NoError(t, model.DB.First(&wallet, "user_id = ? AND currency = ?", 702, "USD").Error)
	assert.Equal(t, int64(2_000_000), wallet.AvailableMicros)

	var count int64
	require.NoError(t, model.DB.Model(&model.MoneyWalletTransaction{}).Where("request_id = ?", "quota_backfill:user:702:USD").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestQuotaWriteGuardRejectsMoneyMode(t *testing.T) {
	setupQuotaMoneyBackfillTest(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 703, Username: "u703", AffCode: "u703", Quota: 0, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode": "money",
	}))

	err := model.IncreaseUserQuota(703, 100, true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrQuotaWriteDisabledInMoneyMode))

	err = model.DecreaseUserQuota(703, 100, true)
	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrQuotaWriteDisabledInMoneyMode))

	var user model.User
	require.NoError(t, model.DB.First(&user, "id = ?", 703).Error)
	assert.Equal(t, 0, user.Quota)
}
