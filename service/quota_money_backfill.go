package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const QuotaBackfillSource = "quota_backfill"

type QuotaMoneyBackfillPreview struct {
	Currency                        string `json:"currency"`
	UserCount                       int64  `json:"user_count"`
	TokenCount                      int64  `json:"token_count"`
	SubscriptionCount               int64  `json:"subscription_count"`
	UserQuotaTotal                  int64  `json:"user_quota_total"`
	TokenRemainQuotaTotal           int64  `json:"token_remain_quota_total"`
	SubscriptionRemainingQuotaTotal int64  `json:"subscription_remaining_quota_total"`
	UserAmountMicrosTotal           int64  `json:"user_amount_micros_total"`
	TokenRemainAmountMicrosTotal    int64  `json:"token_remain_amount_micros_total"`
	SubscriptionAmountMicrosTotal   int64  `json:"subscription_amount_micros_total"`
}

type QuotaMoneyBackfillApplyResult struct {
	Preview                    QuotaMoneyBackfillPreview `json:"preview"`
	UserTransactionsSeen       int64                     `json:"user_transactions_seen"`
	TokenBudgetsUpdated        int64                     `json:"token_budgets_updated"`
	SubscriptionBudgetsUpdated int64                     `json:"subscription_budgets_updated"`
}

func QuotaToMoneyMicros(quota int64) int64 {
	return model.LegacyQuotaToMoneyMicros(quota)
}

func PreviewQuotaMoneyBackfill(currency string) (QuotaMoneyBackfillPreview, error) {
	currency = NormalizeCurrency(currency)
	if currency == "" {
		currency = "USD"
	}
	preview := QuotaMoneyBackfillPreview{Currency: currency}

	if err := model.DB.Model(&model.User{}).
		Where("quota > ?", 0).
		Count(&preview.UserCount).
		Select("COALESCE(SUM(quota), 0)").
		Scan(&preview.UserQuotaTotal).Error; err != nil {
		return preview, err
	}
	if err := model.DB.Model(&model.Token{}).
		Where("remain_quota > ?", 0).
		Count(&preview.TokenCount).
		Select("COALESCE(SUM(remain_quota), 0)").
		Scan(&preview.TokenRemainQuotaTotal).Error; err != nil {
		return preview, err
	}
	if err := model.DB.Model(&model.UserSubscription{}).
		Where("amount_total > amount_used").
		Count(&preview.SubscriptionCount).
		Select("COALESCE(SUM(amount_total - amount_used), 0)").
		Scan(&preview.SubscriptionRemainingQuotaTotal).Error; err != nil {
		return preview, err
	}

	preview.UserAmountMicrosTotal = QuotaToMoneyMicros(preview.UserQuotaTotal)
	preview.TokenRemainAmountMicrosTotal = QuotaToMoneyMicros(preview.TokenRemainQuotaTotal)
	preview.SubscriptionAmountMicrosTotal = QuotaToMoneyMicros(preview.SubscriptionRemainingQuotaTotal)
	return preview, nil
}

func ApplyQuotaMoneyBackfill(currency string) (QuotaMoneyBackfillApplyResult, error) {
	preview, err := PreviewQuotaMoneyBackfill(currency)
	if err != nil {
		return QuotaMoneyBackfillApplyResult{}, err
	}
	result := QuotaMoneyBackfillApplyResult{Preview: preview}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var users []model.User
		if err := tx.Where("quota > ?", 0).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			amountMicros := QuotaToMoneyMicros(int64(user.Quota))
			if amountMicros <= 0 {
				continue
			}
			requestID := fmt.Sprintf("%s:user:%d:%s", QuotaBackfillSource, user.Id, preview.Currency)
			if err := model.CreditWalletWithTx(tx, user.Id, preview.Currency, amountMicros, requestID, model.MoneyWalletTransactionAdjustment, QuotaBackfillSource); err != nil {
				return err
			}
			result.UserTransactionsSeen++
		}
		var tokens []model.Token
		if err := tx.Where("remain_quota > ?", 0).Find(&tokens).Error; err != nil {
			return err
		}
		for _, token := range tokens {
			if token.RemainAmountMicros > 0 {
				continue
			}
			amountMicros := QuotaToMoneyMicros(int64(token.RemainQuota))
			if amountMicros <= 0 {
				continue
			}
			updates := map[string]interface{}{
				"remain_amount_micros": amountMicros,
				"currency":             preview.Currency,
			}
			if err := tx.Model(&model.Token{}).Where("id = ?", token.Id).Updates(updates).Error; err != nil {
				return err
			}
			result.TokenBudgetsUpdated++
		}
		var subscriptions []model.UserSubscription
		if err := tx.Where("amount_total > amount_used").Find(&subscriptions).Error; err != nil {
			return err
		}
		for _, sub := range subscriptions {
			updates := map[string]interface{}{}
			if sub.AmountTotalMicros <= 0 && sub.AmountTotal > 0 {
				updates["amount_total_micros"] = QuotaToMoneyMicros(sub.AmountTotal)
			}
			if sub.AmountUsedMicros <= 0 && sub.AmountUsed > 0 {
				updates["amount_used_micros"] = QuotaToMoneyMicros(sub.AmountUsed)
			}
			if sub.Currency == "" {
				updates["currency"] = preview.Currency
			}
			if len(updates) == 0 {
				continue
			}
			if err := tx.Model(&model.UserSubscription{}).Where("id = ?", sub.Id).Updates(updates).Error; err != nil {
				return err
			}
			result.SubscriptionBudgetsUpdated++
		}
		return nil
	})
	if err != nil {
		return QuotaMoneyBackfillApplyResult{}, err
	}
	return result, nil
}
