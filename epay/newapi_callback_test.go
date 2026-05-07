package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCallbackSenderRequiresSuccessBodyAnd2xxStatus(t *testing.T) {
	cfg := validTestConfig(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	sender := NewCallbackSender(cfg)
	order := &Order{
		OutTradeNo:    "USR1NOabc",
		PaymentType:   "alipay",
		Subject:       "TUC10",
		Amount:        "7.30",
		AlipayTradeNo: "2026050722000000000001",
		NotifyURL:     server.URL,
	}

	err := sender.Send(context.Background(), order)
	require.ErrorContains(t, err, "non-2xx")
}
