package main

import (
	"html"
	"net/url"
	"regexp"
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

func TestAlipayPagePayFormPutsPublicParamsInActionQuery(t *testing.T) {
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

	form, err := client.PagePayForm(order)
	require.NoError(t, err)

	matches := regexp.MustCompile(`action="([^"]+)"`).FindStringSubmatch(form)
	require.Len(t, matches, 2)
	actionURL, err := url.Parse(html.UnescapeString(matches[1]))
	require.NoError(t, err)
	require.Equal(t, "https", actionURL.Scheme)
	require.Equal(t, "openapi.alipay.com", actionURL.Host)

	query := actionURL.Query()
	require.Equal(t, "utf-8", query.Get("charset"))
	require.Equal(t, "RSA2", query.Get("sign_type"))
	require.Equal(t, alipayPagePayMethod, query.Get("method"))
	require.NotEmpty(t, query.Get("sign"))
	require.Empty(t, query.Get("biz_content"))

	require.Contains(t, form, `name="biz_content"`)
	require.NotContains(t, form, `name="charset"`)
	require.NotContains(t, form, `name="sign"`)
}
