package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	MoneyWalletTransactionTopup      = "topup"
	MoneyWalletTransactionPreauth    = "preauth"
	MoneyWalletTransactionSettle     = "settle"
	MoneyWalletTransactionRelease    = "release"
	MoneyWalletTransactionRefund     = "refund"
	MoneyWalletTransactionAdjustment = "adjustment"
	MoneyWalletTransactionGrant      = "grant"
)

var (
	ErrMoneyWalletInsufficientFunds = errors.New("money wallet has insufficient funds")
	ErrMoneyWalletInvalidAmount     = errors.New("money wallet amount is invalid")
	ErrMoneyWalletInvalidRequest    = errors.New("money wallet request id is required")
	ErrMoneyWalletPreauthNotFound   = errors.New("money wallet preauth not found")
	ErrMoneyWalletPreauthClosed     = errors.New("money wallet preauth is already closed")
)

type MoneyWallet struct {
	UserId              int    `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	Currency            string `json:"currency" gorm:"type:varchar(8);primaryKey;autoIncrement:false"`
	AvailableMicros     int64  `json:"available_micros" gorm:"bigint;not null;default:0"`
	FrozenMicros        int64  `json:"frozen_micros" gorm:"bigint;not null;default:0"`
	LifetimeTopupMicros int64  `json:"lifetime_topup_micros" gorm:"bigint;not null;default:0"`
	UpdatedAt           int64  `json:"updated_at" gorm:"bigint;not null;default:0"`
}

type MoneyWalletTransaction struct {
	Id                   int64  `json:"id"`
	UserId               int    `json:"user_id" gorm:"index;not null"`
	Currency             string `json:"currency" gorm:"type:varchar(8);index;not null"`
	Type                 string `json:"type" gorm:"type:varchar(32);not null;uniqueIndex:idx_money_wallet_request_type"`
	RequestId            string `json:"request_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_money_wallet_request_type"`
	ReferenceRequestId   string `json:"reference_request_id" gorm:"type:varchar(128);index"`
	ClosedAt             int64  `json:"closed_at" gorm:"bigint;index"`
	ClosedType           string `json:"closed_type" gorm:"type:varchar(32)"`
	ClosedRequestId      string `json:"closed_request_id" gorm:"type:varchar(128);index"`
	AmountMicros         int64  `json:"amount_micros" gorm:"bigint;not null"`
	AvailableDeltaMicros int64  `json:"available_delta_micros" gorm:"bigint;not null"`
	FrozenDeltaMicros    int64  `json:"frozen_delta_micros" gorm:"bigint;not null"`
	AvailableAfterMicros int64  `json:"available_after_micros" gorm:"bigint;not null"`
	FrozenAfterMicros    int64  `json:"frozen_after_micros" gorm:"bigint;not null"`
	Metadata             string `json:"metadata" gorm:"type:text"`
	CreatedAt            int64  `json:"created_at" gorm:"bigint;index"`
}

func normalizeMoneyCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}

func SettlementCurrency() string {
	currency := normalizeMoneyCurrency(billing_setting.GetMoneyBillingSetting().SettlementCurrency)
	if currency == "" {
		return "USD"
	}
	return currency
}

func ShouldWriteLegacyQuota() bool {
	return !billing_setting.IsMoneyBillingModeEnabled()
}

func LegacyQuotaToMoneyMicros(quota int64) int64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return decimal.NewFromInt(quota).
		Mul(decimal.NewFromInt(1_000_000)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(0).
		IntPart()
}

func legacyQuotaDeltaToMoneyMicros(quota int64) int64 {
	if quota == 0 {
		return 0
	}
	if quota < 0 {
		return -LegacyQuotaToMoneyMicros(-quota)
	}
	return LegacyQuotaToMoneyMicros(quota)
}

func LegacyQuotaDeltaToMoneyMicros(quota int64) int64 {
	return legacyQuotaDeltaToMoneyMicros(quota)
}

func absMicros(amount int64) int64 {
	if amount < 0 {
		return -amount
	}
	return amount
}

func validateMoneyWalletRequest(currency string, requestID string) (string, error) {
	currency = normalizeMoneyCurrency(currency)
	if currency == "" || strings.TrimSpace(requestID) == "" {
		return "", ErrMoneyWalletInvalidRequest
	}
	return currency, nil
}

func ensureMoneyWallet(tx *gorm.DB, userID int, currency string) (*MoneyWallet, error) {
	wallet := &MoneyWallet{}
	query := tx
	if !common.UsingSQLite {
		query = query.Set("gorm:query_option", "FOR UPDATE")
	}
	err := query.Where("user_id = ? AND currency = ?", userID, currency).First(wallet).Error
	if err == nil {
		return wallet, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	wallet = &MoneyWallet{
		UserId:    userID,
		Currency:  currency,
		UpdatedAt: common.GetTimestamp(),
	}
	if err := tx.Create(wallet).Error; err != nil {
		return nil, err
	}
	return wallet, nil
}

func GetMoneyWalletAvailableMicros(userID int, currency string) (int64, error) {
	currency = normalizeMoneyCurrency(currency)
	if currency == "" {
		currency = SettlementCurrency()
	}
	var wallet MoneyWallet
	err := DB.Where("user_id = ? AND currency = ?", userID, currency).First(&wallet).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return wallet.AvailableMicros, nil
}

func existingMoneyWalletTransaction(tx *gorm.DB, requestID string, txType string) (*MoneyWalletTransaction, error) {
	var transaction MoneyWalletTransaction
	err := tx.Where("request_id = ? AND type = ?", requestID, txType).First(&transaction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func hasTerminalMoneyWalletTransaction(tx *gorm.DB, preauthRequestID string) (bool, error) {
	var count int64
	err := tx.Model(&MoneyWalletTransaction{}).
		Where("reference_request_id = ? AND type IN ?", preauthRequestID, []string{MoneyWalletTransactionSettle, MoneyWalletTransactionRelease}).
		Count(&count).Error
	return count > 0, err
}

func closeMoneyWalletPreauth(tx *gorm.DB, preauth *MoneyWalletTransaction, txType string, requestID string) error {
	if preauth.ClosedAt != 0 {
		return ErrMoneyWalletPreauthClosed
	}
	result := tx.Model(&MoneyWalletTransaction{}).
		Where("id = ? AND closed_at = ?", preauth.Id, 0).
		Updates(map[string]interface{}{
			"closed_at":         common.GetTimestamp(),
			"closed_type":       txType,
			"closed_request_id": requestID,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrMoneyWalletPreauthClosed
	}
	return nil
}

func recordMoneyWalletTransaction(tx *gorm.DB, wallet *MoneyWallet, txType string, requestID string, referenceRequestID string, amountMicros int64, availableDelta int64, frozenDelta int64, metadata string) error {
	wallet.AvailableMicros += availableDelta
	wallet.FrozenMicros += frozenDelta
	now := common.GetTimestamp()
	wallet.UpdatedAt = now
	if wallet.AvailableMicros < 0 || wallet.FrozenMicros < 0 {
		return ErrMoneyWalletInsufficientFunds
	}
	if txType == MoneyWalletTransactionTopup && availableDelta > 0 {
		wallet.LifetimeTopupMicros += availableDelta
	}
	if err := tx.Save(wallet).Error; err != nil {
		return err
	}
	return tx.Create(&MoneyWalletTransaction{
		UserId:               wallet.UserId,
		Currency:             wallet.Currency,
		Type:                 txType,
		RequestId:            requestID,
		ReferenceRequestId:   referenceRequestID,
		AmountMicros:         amountMicros,
		AvailableDeltaMicros: availableDelta,
		FrozenDeltaMicros:    frozenDelta,
		AvailableAfterMicros: wallet.AvailableMicros,
		FrozenAfterMicros:    wallet.FrozenMicros,
		Metadata:             metadata,
		CreatedAt:            now,
	}).Error
}

func CreditWallet(userID int, currency string, amountMicros int64, requestID string, txType string, metadata string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return CreditWalletWithTx(tx, userID, currency, amountMicros, requestID, txType, metadata)
	})
}

func CreditWalletWithTx(tx *gorm.DB, userID int, currency string, amountMicros int64, requestID string, txType string, metadata string) error {
	if txType == "" {
		txType = MoneyWalletTransactionTopup
	}
	if amountMicros <= 0 {
		return ErrMoneyWalletInvalidAmount
	}
	currency, err := validateMoneyWalletRequest(currency, requestID)
	if err != nil {
		return err
	}
	existing, err := existingMoneyWalletTransaction(tx, requestID, txType)
	if err != nil || existing != nil {
		return err
	}
	wallet, err := ensureMoneyWallet(tx, userID, currency)
	if err != nil {
		return err
	}
	return recordMoneyWalletTransaction(tx, wallet, txType, requestID, "", amountMicros, amountMicros, 0, metadata)
}

func AdjustWallet(userID int, currency string, amountMicros int64, requestID string, metadata string) error {
	if amountMicros == 0 {
		return ErrMoneyWalletInvalidAmount
	}
	currency, err := validateMoneyWalletRequest(currency, requestID)
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		existing, err := existingMoneyWalletTransaction(tx, requestID, MoneyWalletTransactionAdjustment)
		if err != nil || existing != nil {
			return err
		}
		wallet, err := ensureMoneyWallet(tx, userID, currency)
		if err != nil {
			return err
		}
		return recordMoneyWalletTransaction(tx, wallet, MoneyWalletTransactionAdjustment, requestID, "", amountMicros, amountMicros, 0, metadata)
	})
}

func FreezeWallet(userID int, currency string, amountMicros int64, requestID string, metadata string) error {
	if amountMicros <= 0 {
		return ErrMoneyWalletInvalidAmount
	}
	currency, err := validateMoneyWalletRequest(currency, requestID)
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		existing, err := existingMoneyWalletTransaction(tx, requestID, MoneyWalletTransactionPreauth)
		if err != nil || existing != nil {
			return err
		}
		wallet, err := ensureMoneyWallet(tx, userID, currency)
		if err != nil {
			return err
		}
		if wallet.AvailableMicros < amountMicros {
			return ErrMoneyWalletInsufficientFunds
		}
		return recordMoneyWalletTransaction(tx, wallet, MoneyWalletTransactionPreauth, requestID, "", amountMicros, -amountMicros, amountMicros, metadata)
	})
}

func SettleFrozenWallet(userID int, currency string, preauthRequestID string, amountMicros int64, requestID string, metadata string) error {
	if amountMicros < 0 {
		return ErrMoneyWalletInvalidAmount
	}
	currency, err := validateMoneyWalletRequest(currency, requestID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(preauthRequestID) == "" {
		return ErrMoneyWalletInvalidRequest
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		existing, err := existingMoneyWalletTransaction(tx, requestID, MoneyWalletTransactionSettle)
		if err != nil || existing != nil {
			return err
		}
		closed, err := hasTerminalMoneyWalletTransaction(tx, preauthRequestID)
		if err != nil {
			return err
		}
		if closed {
			return ErrMoneyWalletPreauthClosed
		}
		var preauth MoneyWalletTransaction
		if err := tx.Where("request_id = ? AND type = ?", preauthRequestID, MoneyWalletTransactionPreauth).First(&preauth).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMoneyWalletPreauthNotFound
			}
			return err
		}
		if preauth.UserId != userID || preauth.Currency != currency {
			return fmt.Errorf("%w: preauth does not match wallet", ErrMoneyWalletPreauthNotFound)
		}
		wallet, err := ensureMoneyWallet(tx, userID, currency)
		if err != nil {
			return err
		}
		closed, err = hasTerminalMoneyWalletTransaction(tx, preauthRequestID)
		if err != nil {
			return err
		}
		if closed {
			return ErrMoneyWalletPreauthClosed
		}
		if err := closeMoneyWalletPreauth(tx, &preauth, MoneyWalletTransactionSettle, requestID); err != nil {
			return err
		}
		preauthAmount := preauth.FrozenDeltaMicros
		availableDelta := int64(0)
		if amountMicros < preauthAmount {
			availableDelta = preauthAmount - amountMicros
		} else if amountMicros > preauthAmount {
			availableDelta = -(amountMicros - preauthAmount)
		}
		return recordMoneyWalletTransaction(tx, wallet, MoneyWalletTransactionSettle, requestID, preauthRequestID, amountMicros, availableDelta, -preauthAmount, metadata)
	})
}

func ReleaseFrozenWallet(userID int, currency string, preauthRequestID string, requestID string, metadata string) error {
	currency, err := validateMoneyWalletRequest(currency, requestID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(preauthRequestID) == "" {
		return ErrMoneyWalletInvalidRequest
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		existing, err := existingMoneyWalletTransaction(tx, requestID, MoneyWalletTransactionRelease)
		if err != nil || existing != nil {
			return err
		}
		closed, err := hasTerminalMoneyWalletTransaction(tx, preauthRequestID)
		if err != nil {
			return err
		}
		if closed {
			return ErrMoneyWalletPreauthClosed
		}
		var preauth MoneyWalletTransaction
		if err := tx.Where("request_id = ? AND type = ?", preauthRequestID, MoneyWalletTransactionPreauth).First(&preauth).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMoneyWalletPreauthNotFound
			}
			return err
		}
		if preauth.UserId != userID || preauth.Currency != currency {
			return fmt.Errorf("%w: preauth does not match wallet", ErrMoneyWalletPreauthNotFound)
		}
		wallet, err := ensureMoneyWallet(tx, userID, currency)
		if err != nil {
			return err
		}
		closed, err = hasTerminalMoneyWalletTransaction(tx, preauthRequestID)
		if err != nil {
			return err
		}
		if closed {
			return ErrMoneyWalletPreauthClosed
		}
		if err := closeMoneyWalletPreauth(tx, &preauth, MoneyWalletTransactionRelease, requestID); err != nil {
			return err
		}
		return recordMoneyWalletTransaction(tx, wallet, MoneyWalletTransactionRelease, requestID, preauthRequestID, preauth.FrozenDeltaMicros, preauth.FrozenDeltaMicros, -preauth.FrozenDeltaMicros, metadata)
	})
}

func RefundWallet(userID int, currency string, amountMicros int64, requestID string, metadata string) error {
	return CreditWallet(userID, currency, amountMicros, requestID, MoneyWalletTransactionRefund, metadata)
}

func GrantUserQuotaOrMoneyWithTx(tx *gorm.DB, userID int, quota int, requestID string, metadata string) error {
	if quota <= 0 {
		return nil
	}
	if ShouldWriteLegacyQuota() {
		return tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", quota)).Error
	}
	amountMicros := LegacyQuotaToMoneyMicros(int64(quota))
	if amountMicros <= 0 {
		return nil
	}
	return CreditWalletWithTx(tx, userID, SettlementCurrency(), amountMicros, requestID, MoneyWalletTransactionGrant, metadata)
}

func GrantUserQuotaOrMoney(userID int, quota int, requestID string, metadata string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return GrantUserQuotaOrMoneyWithTx(tx, userID, quota, requestID, metadata)
	})
}

func AdjustWalletLegacyQuota(userID int, quotaDelta int, requestID string, metadata string) error {
	amountMicros := legacyQuotaDeltaToMoneyMicros(int64(quotaDelta))
	if amountMicros == 0 {
		return ErrMoneyWalletInvalidAmount
	}
	return AdjustWallet(userID, SettlementCurrency(), amountMicros, requestID, metadata)
}

func OverrideWalletLegacyQuota(userID int, targetQuota int, requestID string, metadata string) error {
	if targetQuota < 0 {
		return ErrMoneyWalletInvalidAmount
	}
	targetMicros := LegacyQuotaToMoneyMicros(int64(targetQuota))
	return DB.Transaction(func(tx *gorm.DB) error {
		currency := SettlementCurrency()
		wallet, err := ensureMoneyWallet(tx, userID, currency)
		if err != nil {
			return err
		}
		delta := targetMicros - wallet.AvailableMicros
		if delta == 0 {
			return nil
		}
		existing, err := existingMoneyWalletTransaction(tx, requestID, MoneyWalletTransactionAdjustment)
		if err != nil || existing != nil {
			return err
		}
		return recordMoneyWalletTransaction(tx, wallet, MoneyWalletTransactionAdjustment, requestID, "", absMicros(delta), delta, 0, metadata)
	})
}
