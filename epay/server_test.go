package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubmitRejectsInvalidEPaySignature(t *testing.T) {
	cfg := validTestConfig(t)
	store, err := NewFileOrderStore(cfg.OrderStorePath)
	require.NoError(t, err)
	gateway := NewGateway(cfg, store, nil)

	form := signedEPayForm(cfg, nil)
	form.Set("sign", "bad")
	req := httptest.NewRequest(http.MethodPost, "/submit.php", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	gateway.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid sign")
}

func TestSubmitAcceptsSignedOrderAndReturnsAlipayForm(t *testing.T) {
	cfg := validTestConfig(t)
	store, err := NewFileOrderStore(cfg.OrderStorePath)
	require.NoError(t, err)
	gateway := NewGateway(cfg, store, nil)

	form := signedEPayForm(cfg, nil)
	req := httptest.NewRequest(http.MethodPost, "/submit.php", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	gateway.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "alipaysubmit")
	require.Contains(t, rec.Body.String(), "openapi.alipay.com/gateway.do")

	order, ok, err := store.Find("USR1NOabc")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, OrderStatusPaying, order.Status)
}

func TestAlipayNotifyCallbacksNewAPIOnce(t *testing.T) {
	cfg := validTestConfig(t)
	var callbackCount int32
	newAPIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.True(t, VerifyEPayParams(firstFormValues(r.PostForm), cfg.EPayKey))
		require.Equal(t, "USR1NOabc", r.PostForm.Get("out_trade_no"))
		require.Equal(t, "TRADE_SUCCESS", r.PostForm.Get("trade_status"))
		atomic.AddInt32(&callbackCount, 1)
		_, _ = w.Write([]byte("success"))
	}))
	defer newAPIServer.Close()

	store, err := NewFileOrderStore(cfg.OrderStorePath)
	require.NoError(t, err)
	_, _, err = store.UpsertFromSubmit(&Order{
		OutTradeNo:  "USR1NOabc",
		PID:         cfg.EPayPID,
		PaymentType: "alipay",
		Subject:     "TUC10",
		Amount:      "7.30",
		NotifyURL:   newAPIServer.URL,
		ReturnURL:   "https://kittyvibe.ai/console/log",
	})
	require.NoError(t, err)

	gateway := NewGateway(cfg, store, nil)
	notify := signedAlipayNotify(t, cfg, nil)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/alipay/notify", strings.NewReader(notify.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		gateway.Routes().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "success", rec.Body.String())
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&callbackCount))
	order, ok, err := store.Find("USR1NOabc")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, OrderStatusCallbackSuccess, order.Status)
}

func TestAlipayNotifyRejectsAmountMismatch(t *testing.T) {
	cfg := validTestConfig(t)
	store, err := NewFileOrderStore(cfg.OrderStorePath)
	require.NoError(t, err)
	_, _, err = store.UpsertFromSubmit(&Order{
		OutTradeNo:  "USR1NOabc",
		PID:         cfg.EPayPID,
		PaymentType: "alipay",
		Subject:     "TUC10",
		Amount:      "7.30",
		NotifyURL:   "https://kittyvibe.ai/api/user/epay/notify",
		ReturnURL:   "https://kittyvibe.ai/console/log",
	})
	require.NoError(t, err)

	gateway := NewGateway(cfg, store, nil)
	notify := signedAlipayNotify(t, cfg, map[string]string{"total_amount": "7.31"})
	req := httptest.NewRequest(http.MethodPost, "/alipay/notify", strings.NewReader(notify.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	gateway.Routes().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "fail", rec.Body.String())
	order, ok, err := store.Find("USR1NOabc")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, OrderStatusCreated, order.Status)
}
