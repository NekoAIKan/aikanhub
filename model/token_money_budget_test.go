package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTokenMoneyBudgetTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Token{}))
	require.NoError(t, DB.Exec("DELETE FROM tokens").Error)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "legacy",
		"billing_setting.settlement_currency": "USD",
	}))
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		DB.Exec("DELETE FROM tokens")
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
		})
	})
}

func createTokenMoneyBudgetTestToken(t *testing.T, token Token) Token {
	t.Helper()
	if token.Key == "" {
		token.Key = "token-money-budget-key"
	}
	if token.Status == 0 {
		token.Status = common.TokenStatusEnabled
	}
	if token.ExpiredTime == 0 {
		token.ExpiredTime = -1
	}
	require.NoError(t, DB.Create(&token).Error)
	return token
}

func getTokenMoneyBudgetTestToken(t *testing.T, id int) Token {
	t.Helper()
	var token Token
	require.NoError(t, DB.First(&token, "id = ?", id).Error)
	return token
}

func TestTokenMoneyBudgetDecreaseAndIncrease(t *testing.T) {
	setupTokenMoneyBudgetTest(t)
	token := createTokenMoneyBudgetTestToken(t, Token{
		UserId:             801,
		Key:                "token-money-budget-decrease",
		RemainAmountMicros: 1_000_000,
		UsedAmountMicros:   250_000,
		Currency:           "usd",
	})

	require.NoError(t, DecreaseTokenMoneyBudget(token.Id, token.Key, 300_000))
	afterCharge := getTokenMoneyBudgetTestToken(t, token.Id)
	assert.Equal(t, int64(700_000), afterCharge.RemainAmountMicros)
	assert.Equal(t, int64(550_000), afterCharge.UsedAmountMicros)
	assert.Equal(t, "USD", afterCharge.Currency)

	require.NoError(t, IncreaseTokenMoneyBudget(token.Id, token.Key, 200_000))
	afterRefund := getTokenMoneyBudgetTestToken(t, token.Id)
	assert.Equal(t, int64(900_000), afterRefund.RemainAmountMicros)
	assert.Equal(t, int64(350_000), afterRefund.UsedAmountMicros)

	require.NoError(t, IncreaseTokenMoneyBudget(token.Id, token.Key, 500_000))
	afterLargeRefund := getTokenMoneyBudgetTestToken(t, token.Id)
	assert.Equal(t, int64(1_400_000), afterLargeRefund.RemainAmountMicros)
	assert.Equal(t, int64(0), afterLargeRefund.UsedAmountMicros)
}

func TestTokenMoneyBudgetInsufficientLeavesRowUnchanged(t *testing.T) {
	setupTokenMoneyBudgetTest(t)
	token := createTokenMoneyBudgetTestToken(t, Token{
		UserId:             802,
		Key:                "token-money-budget-insufficient",
		RemainAmountMicros: 100_000,
		UsedAmountMicros:   50_000,
	})

	err := DecreaseTokenMoneyBudget(token.Id, token.Key, 100_001)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenMoneyBudgetInsufficient))

	unchanged := getTokenMoneyBudgetTestToken(t, token.Id)
	assert.Equal(t, int64(100_000), unchanged.RemainAmountMicros)
	assert.Equal(t, int64(50_000), unchanged.UsedAmountMicros)
}

func TestTokenQuotaWritesAreRejectedInMoneyMode(t *testing.T) {
	setupTokenMoneyBudgetTest(t)
	token := createTokenMoneyBudgetTestToken(t, Token{
		UserId:      803,
		Key:         "token-money-budget-quota-guard",
		RemainQuota: 100,
		UsedQuota:   50,
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode": "money",
	}))

	err := IncreaseTokenQuota(token.Id, token.Key, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrQuotaWriteDisabledInMoneyMode))

	err = DecreaseTokenQuota(token.Id, token.Key, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrQuotaWriteDisabledInMoneyMode))

	unchanged := getTokenMoneyBudgetTestToken(t, token.Id)
	assert.Equal(t, 100, unchanged.RemainQuota)
	assert.Equal(t, 50, unchanged.UsedQuota)
}

func TestValidateUserTokenUsesMoneyBudgetInMoneyMode(t *testing.T) {
	setupTokenMoneyBudgetTest(t)
	valid := createTokenMoneyBudgetTestToken(t, Token{
		UserId:             804,
		Key:                "token-money-budget-valid",
		RemainQuota:        0,
		RemainAmountMicros: 1,
	})
	exhausted := createTokenMoneyBudgetTestToken(t, Token{
		UserId:             804,
		Key:                "token-money-budget-exhausted",
		RemainQuota:        10_000,
		RemainAmountMicros: 0,
	})
	unlimited := createTokenMoneyBudgetTestToken(t, Token{
		UserId:          804,
		Key:             "token-money-budget-unlimited",
		UnlimitedAmount: true,
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode": "money",
	}))

	loaded, err := ValidateUserToken(valid.Key)
	require.NoError(t, err)
	assert.Equal(t, valid.Id, loaded.Id)

	_, err = ValidateUserToken(exhausted.Key)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTokenInvalid))
	assert.Equal(t, common.TokenStatusExhausted, getTokenMoneyBudgetTestToken(t, exhausted.Id).Status)

	loaded, err = ValidateUserToken(unlimited.Key)
	require.NoError(t, err)
	assert.Equal(t, unlimited.Id, loaded.Id)
}
