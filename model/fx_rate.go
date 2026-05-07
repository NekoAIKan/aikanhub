package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrFxRateNotFound = errors.New("fx rate not found")

type FxRate struct {
	Id            string `json:"id" gorm:"type:varchar(64);primaryKey"`
	BaseCurrency  string `json:"base_currency" gorm:"type:varchar(8);index;not null"`
	QuoteCurrency string `json:"quote_currency" gorm:"type:varchar(8);index;not null"`
	RateMicros    int64  `json:"rate_micros" gorm:"bigint;not null"`
	BufferBps     int64  `json:"buffer_bps" gorm:"bigint;not null;default:0"`
	Source        string `json:"source" gorm:"type:varchar(32);index"`
	EffectiveAt   int64  `json:"effective_at" gorm:"bigint;index"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
}

func (rate *FxRate) normalize() {
	rate.BaseCurrency = strings.ToUpper(strings.TrimSpace(rate.BaseCurrency))
	rate.QuoteCurrency = strings.ToUpper(strings.TrimSpace(rate.QuoteCurrency))
}

func (rate *FxRate) BeforeCreate(tx *gorm.DB) error {
	rate.normalize()
	if rate.CreatedAt == 0 {
		rate.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func (rate *FxRate) BeforeUpdate(tx *gorm.DB) error {
	rate.normalize()
	return nil
}

func GetLatestFxRate(baseCurrency string, quoteCurrency string) (*FxRate, error) {
	base := strings.ToUpper(strings.TrimSpace(baseCurrency))
	quote := strings.ToUpper(strings.TrimSpace(quoteCurrency))
	var rate FxRate
	err := DB.Where("base_currency = ? AND quote_currency = ?", base, quote).
		Order("effective_at DESC, created_at DESC").
		First(&rate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFxRateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rate, nil
}
