package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func validTestConfig(t *testing.T) Config {
	privateKey, publicKey := testRSAKeys(t)
	return Config{
		ListenAddr:            ":0",
		PublicBaseURL:         "https://pay.kittyvibe.ai",
		EPayPID:               "kittyvibe",
		EPayKey:               "epay-secret",
		AllowedNotifyHosts:    map[string]struct{}{"kittyvibe.ai": {}},
		AlipayAppID:           "2021000000000000",
		AlipayAppPrivateKey:   privateKey,
		AlipayPublicKey:       publicKey,
		AlipaySellerID:        "2088000000000000",
		AlipaySandbox:         false,
		AlipayGateway:         "https://openapi.alipay.com/gateway.do",
		OrderStoreType:        OrderStoreTypeFile,
		OrderStorePath:        t.TempDir() + "/orders.json",
		HTTPTimeout:           defaultHTTPTimeout,
		CallbackRetryInterval: defaultCallbackRetryInterval,
	}
}

func testRSAKeys(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	privatePEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))

	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	return privatePEM, publicPEM
}

func signedEPayForm(cfg Config, overrides map[string]string) url.Values {
	params := map[string]string{
		"pid":          cfg.EPayPID,
		"type":         "alipay",
		"out_trade_no": "USR1NOabc",
		"notify_url":   "https://kittyvibe.ai/api/user/epay/notify",
		"return_url":   "https://kittyvibe.ai/console/log",
		"name":         "TUC10",
		"money":        "7.30",
		"device":       "pc",
	}
	for k, v := range overrides {
		params[k] = v
	}
	params = AddEPaySignature(params, cfg.EPayKey)
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	return form
}

func signedAlipayNotify(t *testing.T, cfg Config, overrides map[string]string) url.Values {
	t.Helper()
	params := map[string]string{
		"app_id":       cfg.AlipayAppID,
		"seller_id":    cfg.AlipaySellerID,
		"out_trade_no": "USR1NOabc",
		"trade_no":     "2026050722000000000001",
		"trade_status": "TRADE_SUCCESS",
		"total_amount": "7.30",
		"subject":      "TUC10",
		"sign_type":    "RSA2",
		"charset":      "utf-8",
		"gmt_payment":  "2026-05-07 10:00:00",
		"notify_time":  "2026-05-07 10:00:01",
		"notify_type":  "trade_status_sync",
		"notify_id":    "notify-id",
		"version":      "1.0",
		"buyer_id":     "2088000000000001",
	}
	for k, v := range overrides {
		params[k] = v
	}
	sign, err := signAlipayParams(params, cfg.AlipayAppPrivateKey)
	require.NoError(t, err)
	params["sign"] = sign

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values
}
