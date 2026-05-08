package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignAlipayParamsCoversSignType(t *testing.T) {
	privateKey, publicKey := testRSAKeys(t)
	params := map[string]string{
		"app_id":       "2021000000000000",
		"method":       alipayPagePayMethod,
		"charset":      "utf-8",
		"sign_type":    "RSA2",
		"timestamp":    "2026-05-08 07:37:50",
		"version":      "1.0",
		"biz_content":  `{"out_trade_no":"USR1NOabc","product_code":"FAST_INSTANT_TRADE_PAY","subject":"TUC10","total_amount":"73.00"}`,
		"notify_url":   "https://pay.kittyvibe.ai/alipay/notify",
		"return_url":   "https://pay.kittyvibe.ai/alipay/return",
		"empty_should": "",
	}

	signature, err := signAlipayParams(params, privateKey)
	require.NoError(t, err)

	requireAlipaySignatureValid(t, publicKey, signature, testAlipaySigningContent(params, true))
}

func TestVerifyAlipayParamsAcceptsSignTypeCoveredSignature(t *testing.T) {
	privateKey, publicKey := testRSAKeys(t)
	params := map[string]string{
		"app_id":       "2021000000000000",
		"seller_id":    "2088000000000000",
		"out_trade_no": "USR1NOabc",
		"trade_no":     "2026050722000000000001",
		"trade_status": "TRADE_SUCCESS",
		"total_amount": "7.30",
		"sign_type":    "RSA2",
		"charset":      "utf-8",
		"version":      "1.0",
	}
	params["sign"] = testSignAlipayContent(t, privateKey, testAlipaySigningContent(params, true))

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	parsedPublicKey, err := parseRSAPublicKey(publicKey)
	require.NoError(t, err)
	require.NoError(t, verifyAlipayParamsWithKey(values, parsedPublicKey))
}

func TestVerifyAlipayParamsAcceptsSignatureWithoutSignTypeForCompatibility(t *testing.T) {
	privateKey, publicKey := testRSAKeys(t)
	params := map[string]string{
		"app_id":       "2021000000000000",
		"seller_id":    "2088000000000000",
		"out_trade_no": "USR1NOabc",
		"trade_no":     "2026050722000000000001",
		"trade_status": "TRADE_SUCCESS",
		"total_amount": "7.30",
		"sign_type":    "RSA2",
		"charset":      "utf-8",
		"version":      "1.0",
	}
	params["sign"] = testSignAlipayContent(t, privateKey, testAlipaySigningContent(params, false))

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}

	parsedPublicKey, err := parseRSAPublicKey(publicKey)
	require.NoError(t, err)
	require.NoError(t, verifyAlipayParamsWithKey(values, parsedPublicKey))
}

func testAlipaySigningContent(params map[string]string, includeSignType bool) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || (!includeSignType && k == "sign_type") || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func testSignAlipayContent(t *testing.T, privateKeyRaw string, content string) string {
	t.Helper()
	privateKey, err := parseRSAPrivateKey(privateKeyRaw)
	require.NoError(t, err)

	digest := sha256.Sum256([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(signature)
}

func requireAlipaySignatureValid(t *testing.T, publicKeyRaw string, signatureRaw string, content string) {
	t.Helper()
	publicKey, err := parseRSAPublicKey(publicKeyRaw)
	require.NoError(t, err)
	signature, err := base64.StdEncoding.DecodeString(signatureRaw)
	require.NoError(t, err)

	digest := sha256.Sum256([]byte(content))
	require.NoError(t, rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature))
}
