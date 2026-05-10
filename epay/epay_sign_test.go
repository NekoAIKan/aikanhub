package main

import (
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/stretchr/testify/require"
)

func TestSignEPayParamsMatchesGoEpay(t *testing.T) {
	params := map[string]string{
		"pid":          "kittyvibe",
		"type":         "alipay",
		"out_trade_no": "USR1NOabc",
		"notify_url":   "https://kittyvibe.ai/api/user/epay/notify",
		"return_url":   "https://kittyvibe.ai/usage-logs/common",
		"name":         "TUC10",
		"money":        "7.30",
		"device":       "pc",
		"sign_type":    "MD5",
		"sign":         "",
	}

	got := AddEPaySignature(params, "secret")
	want := epay.GenerateParams(params, "secret")

	require.Equal(t, want["sign"], got["sign"])
	require.True(t, VerifyEPayParams(got, "secret"))
	got["money"] = "7.31"
	require.False(t, VerifyEPayParams(got, "secret"))
}
