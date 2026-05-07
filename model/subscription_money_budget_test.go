package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionMoneyBudgetTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}))
	if common.UsingSQLite {
		require.NoError(t, ensureSubscriptionPlanTableSQLite())
	} else {
		require.NoError(t, DB.AutoMigrate(&SubscriptionPlan{}))
	}
	for _, table := range []string{
		"subscription_pre_consume_records",
		"user_subscriptions",
		"subscription_plans",
		"users",
	} {
		require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
	}
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "legacy",
		"billing_setting.settlement_currency": "USD",
	}))
	t.Cleanup(func() {
		for _, table := range []string{
			"subscription_pre_consume_records",
			"user_subscriptions",
			"subscription_plans",
			"users",
		} {
			DB.Exec("DELETE FROM " + table)
		}
		common.QuotaPerUnit = originalQuotaPerUnit
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
		})
	})
}

func setSubscriptionMoneyMode(t *testing.T) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "money",
		"billing_setting.settlement_currency": "USD",
	}))
}

func TestCreateUserSubscriptionFromPlanSetsMoneyBudget(t *testing.T) {
	setupSubscriptionMoneyBudgetTest(t)
	setSubscriptionMoneyMode(t)
	require.NoError(t, DB.Create(&User{Id: 9301, Username: "sub-money-create", AffCode: "smc01", Status: common.UserStatusEnabled}).Error)

	plan := &SubscriptionPlan{
		Title:             "Money plan",
		PriceAmount:       10,
		Currency:          "usd",
		DurationUnit:      SubscriptionDurationDay,
		DurationValue:     1,
		TotalAmount:       500_000,
		TotalAmountMicros: 3_000_000,
	}
	require.NoError(t, DB.Create(plan).Error)

	var sub *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		sub, err = CreateUserSubscriptionFromPlanTx(tx, 9301, plan, "test")
		return err
	}))

	assert.Equal(t, int64(500_000), sub.AmountTotal)
	assert.Equal(t, int64(0), sub.AmountUsed)
	assert.Equal(t, int64(3_000_000), sub.AmountTotalMicros)
	assert.Equal(t, int64(0), sub.AmountUsedMicros)
	assert.Equal(t, "USD", sub.Currency)
}

func TestCreateUserSubscriptionFromPlanConvertsLegacyQuotaBudget(t *testing.T) {
	setupSubscriptionMoneyBudgetTest(t)
	setSubscriptionMoneyMode(t)
	require.NoError(t, DB.Create(&User{Id: 9302, Username: "sub-money-convert", AffCode: "smc02", Status: common.UserStatusEnabled}).Error)

	plan := &SubscriptionPlan{
		Title:         "Converted plan",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
		TotalAmount:   750_000,
	}
	require.NoError(t, DB.Create(plan).Error)

	var sub *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		sub, err = CreateUserSubscriptionFromPlanTx(tx, 9302, plan, "test")
		return err
	}))

	assert.Equal(t, int64(750_000), sub.AmountTotal)
	assert.Equal(t, int64(1_500_000), sub.AmountTotalMicros)
	assert.Equal(t, "USD", sub.Currency)
}

func TestPreConsumeUserSubscriptionUsesMoneyBudgetInMoneyMode(t *testing.T) {
	setupSubscriptionMoneyBudgetTest(t)
	setSubscriptionMoneyMode(t)
	now := GetDBTimestamp()
	require.NoError(t, DB.Create(&User{Id: 9303, Username: "sub-money-preconsume", AffCode: "smp01", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&SubscriptionPlan{
		Id:            9303,
		Title:         "Sub money",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationDay,
		DurationValue: 1,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:            9303,
		PlanId:            9303,
		AmountTotal:       999,
		AmountUsed:        0,
		AmountTotalMicros: 1_500_000,
		AmountUsedMicros:  0,
		Currency:          "USD",
		StartTime:         now - 10,
		EndTime:           now + 3600,
		Status:            "active",
	}).Error)

	res, err := PreConsumeUserSubscription("sub-money-preconsume-1", 9303, "gpt-test", 0, 500_000)
	require.NoError(t, err)

	assert.Equal(t, int64(500_000), res.PreConsumed)
	assert.Equal(t, int64(1_000_000), res.PreConsumedAmountMicros)
	assert.Equal(t, int64(1_500_000), res.AmountTotalMicros)
	assert.Equal(t, int64(0), res.AmountUsedMicrosBefore)
	assert.Equal(t, int64(1_000_000), res.AmountUsedMicrosAfter)
	assert.Equal(t, "USD", res.Currency)

	var sub UserSubscription
	require.NoError(t, DB.First(&sub, "id = ?", res.UserSubscriptionId).Error)
	assert.Equal(t, int64(0), sub.AmountUsed)
	assert.Equal(t, int64(1_000_000), sub.AmountUsedMicros)

	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.First(&record, "request_id = ?", "sub-money-preconsume-1").Error)
	assert.Equal(t, int64(500_000), record.PreConsumed)
	assert.Equal(t, int64(1_000_000), record.PreConsumedAmountMicros)
	assert.Equal(t, "USD", record.Currency)
}

func TestPostConsumeUserSubscriptionDeltaUsesMoneyBudgetInMoneyMode(t *testing.T) {
	setupSubscriptionMoneyBudgetTest(t)
	setSubscriptionMoneyMode(t)
	now := GetDBTimestamp()
	require.NoError(t, DB.Create(&User{Id: 9304, Username: "sub-money-delta", AffCode: "smd01", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		UserId:            9304,
		PlanId:            0,
		AmountTotal:       100,
		AmountUsed:        10,
		AmountTotalMicros: 3_000_000,
		AmountUsedMicros:  1_000_000,
		Currency:          "USD",
		StartTime:         now - 10,
		EndTime:           now + 3600,
		Status:            "active",
	}).Error)
	var sub UserSubscription
	require.NoError(t, DB.First(&sub, "user_id = ?", 9304).Error)

	require.NoError(t, PostConsumeUserSubscriptionDelta(sub.Id, 250_000))
	require.NoError(t, DB.First(&sub, "id = ?", sub.Id).Error)
	assert.Equal(t, int64(10), sub.AmountUsed)
	assert.Equal(t, int64(1_500_000), sub.AmountUsedMicros)

	require.NoError(t, PostConsumeUserSubscriptionDelta(sub.Id, -750_000))
	require.NoError(t, DB.First(&sub, "id = ?", sub.Id).Error)
	assert.Equal(t, int64(10), sub.AmountUsed)
	assert.Equal(t, int64(0), sub.AmountUsedMicros)
}
