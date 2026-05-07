package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFxRateTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&FxRate{}))
	require.NoError(t, DB.Exec("DELETE FROM fx_rates").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM fx_rates")
	})
}

func TestGetLatestFxRateReturnsMostRecentEffectiveRate(t *testing.T) {
	setupFxRateTest(t)
	require.NoError(t, DB.Create(&FxRate{
		Id:            "old",
		BaseCurrency:  "cny",
		QuoteCurrency: "usd",
		RateMicros:    130000,
		EffectiveAt:   100,
	}).Error)
	require.NoError(t, DB.Create(&FxRate{
		Id:            "new",
		BaseCurrency:  "CNY",
		QuoteCurrency: "USD",
		RateMicros:    137000,
		EffectiveAt:   200,
	}).Error)

	rate, err := GetLatestFxRate("cny", "usd")
	require.NoError(t, err)
	assert.Equal(t, "new", rate.Id)
	assert.Equal(t, "CNY", rate.BaseCurrency)
	assert.Equal(t, "USD", rate.QuoteCurrency)
	assert.Equal(t, int64(137000), rate.RateMicros)
}
