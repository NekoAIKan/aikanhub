package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPostgresOrderStoreContract(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	store, err := NewPostgresOrderStore(databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	prefix := fmt.Sprintf("TEST%d", time.Now().UnixNano())
	t.Cleanup(func() {
		store.db.Exec("DELETE FROM epay_order_events WHERE out_trade_no LIKE ?", prefix+"%")
		store.db.Exec("DELETE FROM epay_orders WHERE out_trade_no LIKE ?", prefix+"%")
	})

	order := &Order{
		OutTradeNo:  prefix + "A",
		PID:         "kittyvibe",
		PaymentType: "alipay",
		Subject:     "TUC10",
		Amount:      "7.30",
		NotifyURL:   "https://kittyvibe.ai/api/user/epay/notify",
		ReturnURL:   "https://kittyvibe.ai/console/log",
	}

	stored, existed, err := store.UpsertFromSubmit(order)
	require.NoError(t, err)
	require.False(t, existed)
	require.Equal(t, OrderStatusCreated, stored.Status)

	stored, existed, err = store.UpsertFromSubmit(order)
	require.NoError(t, err)
	require.True(t, existed)
	require.Equal(t, OrderStatusCreated, stored.Status)

	changed := cloneOrder(order)
	changed.Amount = "7.31"
	_, _, err = store.UpsertFromSubmit(changed)
	require.ErrorContains(t, err, "different fields")

	require.NoError(t, store.MarkPaying(order.OutTradeNo))
	stored, ok, err := store.Find(order.OutTradeNo)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, OrderStatusPaying, stored.Status)

	paid, shouldCallback, err := store.MarkPaid(order.OutTradeNo, "2026050722000000000001")
	require.NoError(t, err)
	require.True(t, shouldCallback)
	require.Equal(t, OrderStatusCallbackPending, paid.Status)

	_, shouldCallback, err = store.MarkPaid(order.OutTradeNo, "2026050722000000000001")
	require.NoError(t, err)
	require.False(t, shouldCallback)

	require.NoError(t, store.RecordCallback(order.OutTradeNo, false, "temporary failure"))
	retryable, err := store.RetryableOrders(defaultMaxCallbackAttempts)
	require.NoError(t, err)
	require.Len(t, retryable, 1)
	require.Equal(t, order.OutTradeNo, retryable[0].OutTradeNo)

	require.NoError(t, store.RecordCallback(order.OutTradeNo, true, ""))
	retryable, err = store.RetryableOrders(defaultMaxCallbackAttempts)
	require.NoError(t, err)
	require.Empty(t, retryable)

	var eventCount int64
	require.NoError(t, store.db.Model(&postgresOrderEvent{}).
		Where("out_trade_no = ?", order.OutTradeNo).
		Count(&eventCount).Error)
	require.GreaterOrEqual(t, eventCount, int64(4))
}
