package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// ---------------------------------------------------------------------------
// FundingSource — 资金来源接口（钱包 or 订阅）
// ---------------------------------------------------------------------------

// FundingSource 抽象了预扣费的资金来源。
type FundingSource interface {
	// Source 返回资金来源标识："wallet" 或 "subscription"
	Source() string
	// PreConsume 从该资金来源预扣 amount 额度
	PreConsume(amount int) error
	// Settle 根据差额调整资金来源（正数补扣，负数退还）
	Settle(delta int) error
	// Refund 退还所有预扣费
	Refund() error
}

// ---------------------------------------------------------------------------
// WalletFunding — 钱包资金来源实现
// ---------------------------------------------------------------------------

type WalletFunding struct {
	userId           int
	requestId        string
	currency         string
	preauthRequestId string
	consumed         int // 实际预扣的用户额度
}

func (w *WalletFunding) Source() string { return BillingSourceWallet }

func NewWalletFunding(userID int, requestID string) *WalletFunding {
	if requestID == "" {
		requestID = common.GetUUID()
	}
	return &WalletFunding{
		userId:    userID,
		requestId: requestID,
		currency:  model.SettlementCurrency(),
	}
}

func (w *WalletFunding) preauthID() string {
	if w.preauthRequestId == "" {
		w.preauthRequestId = fmt.Sprintf("billing:%s:wallet:preauth", w.requestId)
	}
	return w.preauthRequestId
}

func (w *WalletFunding) requestScopedID(suffix string) string {
	return fmt.Sprintf("billing:%s:wallet:%s", w.requestId, suffix)
}

func (w *WalletFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if !model.ShouldWriteLegacyQuota() {
		amountMicros := model.LegacyQuotaToMoneyMicros(int64(amount))
		if amountMicros <= 0 {
			return nil
		}
		if err := model.FreezeWallet(w.userId, w.currency, amountMicros, w.preauthID(), "billing_preconsume"); err != nil {
			return err
		}
		w.consumed = amount
		return nil
	}
	if err := model.DecreaseUserQuota(w.userId, amount, false); err != nil {
		return err
	}
	w.consumed = amount
	return nil
}

func (w *WalletFunding) Settle(delta int) error {
	if delta == 0 {
		if !model.ShouldWriteLegacyQuota() && w.preauthRequestId != "" {
			return model.SettleFrozenWallet(w.userId, w.currency, w.preauthID(), model.LegacyQuotaToMoneyMicros(int64(w.consumed)), w.requestScopedID("settle"), "billing_settle")
		}
		return nil
	}
	if !model.ShouldWriteLegacyQuota() {
		finalQuota := w.consumed + delta
		if finalQuota < 0 {
			finalQuota = 0
		}
		finalMicros := model.LegacyQuotaToMoneyMicros(int64(finalQuota))
		if w.preauthRequestId == "" {
			if finalMicros <= 0 {
				return nil
			}
			return model.AdjustWallet(w.userId, w.currency, -finalMicros, w.requestScopedID("settle-direct"), "billing_settle_direct")
		}
		return model.SettleFrozenWallet(w.userId, w.currency, w.preauthID(), finalMicros, w.requestScopedID("settle"), "billing_settle")
	}
	if delta > 0 {
		return model.DecreaseUserQuota(w.userId, delta, false)
	}
	return model.IncreaseUserQuota(w.userId, -delta, false)
}

func (w *WalletFunding) Refund() error {
	if w.consumed <= 0 {
		return nil
	}
	if !model.ShouldWriteLegacyQuota() {
		if w.preauthRequestId == "" {
			return nil
		}
		return model.ReleaseFrozenWallet(w.userId, w.currency, w.preauthID(), w.requestScopedID("release"), "billing_refund")
	}
	// IncreaseUserQuota 是 quota += N 的非幂等操作，不能重试，否则会多退额度。
	// 订阅的 RefundSubscriptionPreConsume 有 requestId 幂等保护所以可以重试。
	return model.IncreaseUserQuota(w.userId, w.consumed, false)
}

// ---------------------------------------------------------------------------
// SubscriptionFunding — 订阅资金来源实现
// ---------------------------------------------------------------------------

type SubscriptionFunding struct {
	requestId      string
	userId         int
	modelName      string
	amount         int64 // 预扣的订阅额度（subConsume）
	subscriptionId int
	preConsumed    int64
	// 以下字段在 PreConsume 成功后填充，供 RelayInfo 同步使用
	AmountTotal           int64
	AmountUsedAfter       int64
	PlanId                int
	PlanTitle             string
	AmountTotalMicros     int64
	AmountUsedMicrosAfter int64
	Currency              string
}

func (s *SubscriptionFunding) Source() string { return BillingSourceSubscription }

func (s *SubscriptionFunding) PreConsume(_ int) error {
	// amount 参数被忽略，使用内部 s.amount（已在构造时根据 preConsumedQuota 计算）
	res, err := model.PreConsumeUserSubscription(s.requestId, s.userId, s.modelName, 0, s.amount)
	if err != nil {
		return err
	}
	s.subscriptionId = res.UserSubscriptionId
	s.preConsumed = res.PreConsumed
	s.AmountTotal = res.AmountTotal
	s.AmountUsedAfter = res.AmountUsedAfter
	s.AmountTotalMicros = res.AmountTotalMicros
	s.AmountUsedMicrosAfter = res.AmountUsedMicrosAfter
	s.Currency = res.Currency
	// 获取订阅计划信息
	if planInfo, err := model.GetSubscriptionPlanInfoByUserSubscriptionId(res.UserSubscriptionId); err == nil && planInfo != nil {
		s.PlanId = planInfo.PlanId
		s.PlanTitle = planInfo.PlanTitle
	}
	return nil
}

func (s *SubscriptionFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	return model.PostConsumeUserSubscriptionDelta(s.subscriptionId, int64(delta))
}

func (s *SubscriptionFunding) Refund() error {
	if s.preConsumed <= 0 {
		return nil
	}
	return refundWithRetry(func() error {
		return model.RefundSubscriptionPreConsume(s.requestId)
	})
}

// refundWithRetry 尝试多次执行退款操作以提高成功率，只能用于基于事务的退款函数！！！！！！
// try to refund with retries, only for refund functions based on transactions!!!
func refundWithRetry(fn func() error) error {
	if fn == nil {
		return nil
	}
	const maxAttempts = 3
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxAttempts-1 {
			time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
		}
	}
	return lastErr
}
