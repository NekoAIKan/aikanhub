package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

const MoneyMicrosPerUnit int64 = 1_000_000

var (
	ErrInvalidMoneyAmount = errors.New("invalid money amount")
	ErrMissingFxRate      = errors.New("missing fx rate")
)

type FxConversion struct {
	FxRateID     string
	RateMicros   int64
	BufferBps    int64
	FromCurrency string
	ToCurrency   string
}

func NormalizeCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func ApplyBps(amountMicros int64, bps int64) int64 {
	if amountMicros == 0 || bps == 0 {
		return amountMicros
	}
	return amountMicros * (10_000 + bps) / 10_000
}

func ConvertMicros(amountMicros int64, fromCurrency string, toCurrency string, bufferBps int64) (int64, FxConversion, error) {
	if amountMicros < 0 {
		return 0, FxConversion{}, ErrInvalidMoneyAmount
	}
	from := NormalizeCurrency(fromCurrency)
	to := NormalizeCurrency(toCurrency)
	if from == "" || to == "" {
		return 0, FxConversion{}, fmt.Errorf("%w: currency is required", ErrInvalidMoneyAmount)
	}
	if from == to {
		converted := ApplyBps(amountMicros, bufferBps)
		return converted, FxConversion{
			FxRateID:     fmt.Sprintf("same:%s", from),
			RateMicros:   MoneyMicrosPerUnit,
			BufferBps:    bufferBps,
			FromCurrency: from,
			ToCurrency:   to,
		}, nil
	}

	rate, err := model.GetLatestFxRate(from, to)
	if err != nil {
		return 0, FxConversion{}, fmt.Errorf("%w: %s to %s", ErrMissingFxRate, from, to)
	}
	converted := amountMicros * rate.RateMicros / MoneyMicrosPerUnit
	converted = ApplyBps(converted, bufferBps+rate.BufferBps)
	return converted, FxConversion{
		FxRateID:     rate.Id,
		RateMicros:   rate.RateMicros,
		BufferBps:    bufferBps + rate.BufferBps,
		FromCurrency: from,
		ToCurrency:   to,
	}, nil
}
