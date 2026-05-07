package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMoneyBillingSessionTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}, &model.MoneyWallet{}, &model.MoneyWalletTransaction{}))
	for _, table := range []string{"money_wallet_transactions", "money_wallets", "tokens", "users"} {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "money",
		"billing_setting.settlement_currency": "USD",
	}))
	t.Cleanup(func() {
		for _, table := range []string{"money_wallet_transactions", "money_wallets", "tokens", "users"} {
			model.DB.Exec("DELETE FROM " + table)
		}
		common.QuotaPerUnit = originalQuotaPerUnit
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
		})
	})
}

func seedMoneyBillingSessionUserAndToken(t *testing.T, userID int, tokenKey string) model.Token {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{Id: userID, Username: tokenKey, AffCode: tokenKey[:4], Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.CreditWallet(userID, "USD", 2_000_000, "topup:"+tokenKey, model.MoneyWalletTransactionTopup, "test"))
	token := model.Token{
		UserId:             userID,
		Key:                tokenKey,
		Status:             common.TokenStatusEnabled,
		Name:               tokenKey,
		ExpiredTime:        -1,
		RemainAmountMicros: 2_000_000,
		Currency:           "USD",
	}
	require.NoError(t, model.DB.Create(&token).Error)
	return token
}

func newMoneyBillingSessionContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	return ctx
}

func newMoneyBillingRelayInfo(userID int, token model.Token, requestID string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		UserId:          userID,
		TokenId:         token.Id,
		TokenKey:        token.Key,
		RequestId:       requestID,
		OriginModelName: "gpt-money-test",
		UserSetting:     dto.UserSetting{BillingPreference: "wallet_only"},
	}
}

func fetchMoneyBillingSessionWallet(t *testing.T, userID int) model.MoneyWallet {
	t.Helper()
	var wallet model.MoneyWallet
	require.NoError(t, model.DB.First(&wallet, "user_id = ? AND currency = ?", userID, "USD").Error)
	return wallet
}

func fetchMoneyBillingSessionToken(t *testing.T, id int) model.Token {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.First(&token, "id = ?", id).Error)
	return token
}

func TestMoneyBillingSessionPreconsumeAndSettleExact(t *testing.T) {
	setupMoneyBillingSessionTest(t)
	token := seedMoneyBillingSessionUserAndToken(t, 1001, "money-session-exact")
	relayInfo := newMoneyBillingRelayInfo(1001, token, "req-money-session-exact")
	ctx := newMoneyBillingSessionContext()

	apiErr := PreConsumeBilling(ctx, 500_000, relayInfo)
	require.Nil(t, apiErr)

	wallet := fetchMoneyBillingSessionWallet(t, 1001)
	assert.Equal(t, int64(1_000_000), wallet.AvailableMicros)
	assert.Equal(t, int64(1_000_000), wallet.FrozenMicros)
	prechargedToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_000_000), prechargedToken.RemainAmountMicros)
	assert.Equal(t, int64(1_000_000), prechargedToken.UsedAmountMicros)

	require.NoError(t, SettleBilling(ctx, relayInfo, 500_000))

	wallet = fetchMoneyBillingSessionWallet(t, 1001)
	assert.Equal(t, int64(1_000_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	settledToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_000_000), settledToken.RemainAmountMicros)
	assert.Equal(t, int64(1_000_000), settledToken.UsedAmountMicros)
}

func TestMoneyBillingSessionSettleRefundsDifference(t *testing.T) {
	setupMoneyBillingSessionTest(t)
	token := seedMoneyBillingSessionUserAndToken(t, 1002, "money-session-refund")
	relayInfo := newMoneyBillingRelayInfo(1002, token, "req-money-session-refund")
	ctx := newMoneyBillingSessionContext()

	apiErr := PreConsumeBilling(ctx, 500_000, relayInfo)
	require.Nil(t, apiErr)
	require.NoError(t, SettleBilling(ctx, relayInfo, 250_000))

	wallet := fetchMoneyBillingSessionWallet(t, 1002)
	assert.Equal(t, int64(1_500_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	settledToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(1_500_000), settledToken.RemainAmountMicros)
	assert.Equal(t, int64(500_000), settledToken.UsedAmountMicros)
}

func TestMoneyBillingSessionSettleChargesDifference(t *testing.T) {
	setupMoneyBillingSessionTest(t)
	token := seedMoneyBillingSessionUserAndToken(t, 1003, "money-session-charge")
	relayInfo := newMoneyBillingRelayInfo(1003, token, "req-money-session-charge")
	ctx := newMoneyBillingSessionContext()

	apiErr := PreConsumeBilling(ctx, 500_000, relayInfo)
	require.Nil(t, apiErr)
	require.NoError(t, SettleBilling(ctx, relayInfo, 750_000))

	wallet := fetchMoneyBillingSessionWallet(t, 1003)
	assert.Equal(t, int64(500_000), wallet.AvailableMicros)
	assert.Equal(t, int64(0), wallet.FrozenMicros)
	settledToken := fetchMoneyBillingSessionToken(t, token.Id)
	assert.Equal(t, int64(500_000), settledToken.RemainAmountMicros)
	assert.Equal(t, int64(1_500_000), settledToken.UsedAmountMicros)
}
