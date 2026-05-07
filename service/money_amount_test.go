package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMoneyAmountFxTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.FxRate{}))
	require.NoError(t, model.DB.Exec("DELETE FROM fx_rates").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM fx_rates")
	})
}

func TestApplyBps(t *testing.T) {
	assert.Equal(t, int64(1_050_000), ApplyBps(1_000_000, 500))
	assert.Equal(t, int64(950_000), ApplyBps(1_000_000, -500))
	assert.Equal(t, int64(1_000_000), ApplyBps(1_000_000, 0))
}

func TestConvertMicrosSameCurrency(t *testing.T) {
	amount, conversion, err := ConvertMicros(2_000_000, "usd", "USD", 250)
	require.NoError(t, err)
	assert.Equal(t, int64(2_050_000), amount)
	assert.Equal(t, "same:USD", conversion.FxRateID)
	assert.Equal(t, int64(1_000_000), conversion.RateMicros)
	assert.Equal(t, int64(250), conversion.BufferBps)
}

func TestConvertMicrosCrossCurrency(t *testing.T) {
	setupMoneyAmountFxTest(t)
	require.NoError(t, model.DB.Create(&model.FxRate{
		Id:            "fx-cny-usd",
		BaseCurrency:  "CNY",
		QuoteCurrency: "USD",
		RateMicros:    137000,
		BufferBps:     500,
		Source:        "manual",
		EffectiveAt:   100,
	}).Error)

	amount, conversion, err := ConvertMicros(10_000_000, "CNY", "USD", 250)
	require.NoError(t, err)
	assert.Equal(t, int64(1_472_750), amount)
	assert.Equal(t, "fx-cny-usd", conversion.FxRateID)
	assert.Equal(t, int64(137000), conversion.RateMicros)
	assert.Equal(t, int64(750), conversion.BufferBps)
}

func TestConvertMicrosMissingRate(t *testing.T) {
	setupMoneyAmountFxTest(t)
	_, _, err := ConvertMicros(10_000_000, "CNY", "USD", 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMissingFxRate))
}
