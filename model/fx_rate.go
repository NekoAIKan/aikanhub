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

func ListFxRates(offset int, limit int) ([]FxRate, int64, error) {
	var rates []FxRate
	var total int64
	query := DB.Model(&FxRate{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = DB.Order("effective_at DESC, created_at DESC")
	if limit > 0 {
		query = query.Offset(offset).Limit(limit)
	}
	if err := query.Find(&rates).Error; err != nil {
		return nil, 0, err
	}
	return rates, total, nil
}

func GetFxRateById(id string) (*FxRate, error) {
	var rate FxRate
	err := DB.First(&rate, "id = ?", strings.TrimSpace(id)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrFxRateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

func DeleteFxRateById(id string) error {
	result := DB.Delete(&FxRate{}, "id = ?", strings.TrimSpace(id))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrFxRateNotFound
	}
	return nil
}
