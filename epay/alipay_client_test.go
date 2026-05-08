package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAlipayPagePayParamsAreSignedAndContainGatewayCallbacks(t *testing.T) {
	cfg := validTestConfig(t)
	client := NewAlipayClient(cfg)
	client.now = func() time.Time {
		return time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	}
	order := &Order{
		OutTradeNo: "USR1NOabc",
		Subject:    "TUC10",
		Amount:     "7.30",
	}

	params, err := client.PagePayParams(order)
	require.NoError(t, err)

	require.Equal(t, "2021000000000000", params["app_id"])
	require.Equal(t, alipayPagePayMethod, params["method"])
	require.Equal(t, "https://pay.kittyvibe.ai/alipay/notify", params["notify_url"])
	require.Equal(t, "https://pay.kittyvibe.ai/alipay/return", params["return_url"])
	require.Contains(t, params["biz_content"], `"out_trade_no":"USR1NOabc"`)
	require.Contains(t, params["biz_content"], `"total_amount":"7.30"`)
	require.NotEmpty(t, params["sign"])
}
