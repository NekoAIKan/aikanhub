package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/shopspring/decimal"
)

const maxFormBodyBytes int64 = 64 * 1024

type Gateway struct {
	cfg             Config
	store           OrderStore
	alipay          *AlipayClient
	alipayPublicKey *rsa.PublicKey
	callback        *CallbackSender
	logger          *log.Logger
}

func NewGateway(cfg Config, store OrderStore, logger *log.Logger) (*Gateway, error) {
	if logger == nil {
		logger = log.Default()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	keys, err := NewAlipayKeyPair(cfg)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		cfg:             cfg,
		store:           store,
		alipay:          NewAlipayClientWithKey(cfg, keys.Private),
		alipayPublicKey: keys.Public,
		callback:        NewCallbackSender(cfg),
		logger:          logger,
	}, nil
}

func (g *Gateway) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", g.handleHealth)
	mux.HandleFunc("/submit.php", g.handleSubmit)
	mux.HandleFunc("/alipay/notify", g.handleAlipayNotify)
	mux.HandleFunc("/alipay/return", g.handleAlipayReturn)
	return mux
}

func (g *Gateway) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (g *Gateway) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := parseLimitedForm(w, r); err != nil {
		writeFormError(w, err, "invalid form")
		return
	}
	params := firstFormValues(r.PostForm)
	if params["pid"] != g.cfg.EPayPID {
		http.Error(w, "invalid pid", http.StatusBadRequest)
		return
	}
	if params["type"] != "alipay" {
		http.Error(w, "unsupported payment type", http.StatusBadRequest)
		return
	}
	if params["sign_type"] != "MD5" || !VerifyEPayParams(params, g.cfg.EPayKey) {
		http.Error(w, "invalid sign", http.StatusBadRequest)
		return
	}
	notifyURL, err := g.cfg.IsAllowedCallbackURL(params["notify_url"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	returnURL, err := g.cfg.IsAllowedCallbackURL(params["return_url"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	amount, err := normalizeAmount(params["money"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(params["out_trade_no"]) == "" || strings.TrimSpace(params["name"]) == "" {
		http.Error(w, "missing order fields", http.StatusBadRequest)
		return
	}
	order := &Order{
		OutTradeNo:  params["out_trade_no"],
		PID:         params["pid"],
		PaymentType: params["type"],
		Subject:     params["name"],
		Amount:      amount,
		NotifyURL:   notifyURL.String(),
		ReturnURL:   returnURL.String(),
	}
	stored, _, err := g.store.UpsertFromSubmit(order)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	form, err := g.alipay.PagePayForm(stored)
	if err != nil {
		g.logger.Printf("create Alipay form failed order=%s error=%v", stored.OutTradeNo, err)
		http.Error(w, "create payment failed", http.StatusInternalServerError)
		return
	}
	if err := g.store.MarkPaying(stored.OutTradeNo); err != nil {
		g.logger.Printf("mark paying failed order=%s error=%v", stored.OutTradeNo, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(form))
}

func (g *Gateway) handleAlipayNotify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := parseLimitedForm(w, r); err != nil {
		if isRequestBodyTooLarge(err) {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = w.Write([]byte("fail"))
		return
	}
	notification, err := VerifyAlipayNotificationWithKey(r.PostForm, g.cfg, g.alipayPublicKey)
	if err != nil {
		g.logger.Printf("Alipay notify verify failed error=%v", err)
		_, _ = w.Write([]byte("fail"))
		return
	}
	order, ok, err := g.store.Find(notification.OutTradeNo)
	if err != nil || !ok {
		g.logger.Printf("Alipay notify unknown order=%s error=%v", notification.OutTradeNo, err)
		_, _ = w.Write([]byte("fail"))
		return
	}
	if notification.TotalAmount != order.Amount {
		g.logger.Printf("Alipay amount mismatch order=%s expected=%s got=%s", order.OutTradeNo, order.Amount, notification.TotalAmount)
		_, _ = w.Write([]byte("fail"))
		return
	}
	if !notification.IsPaid() {
		g.logger.Printf("Alipay notify ignored order=%s status=%s", order.OutTradeNo, notification.TradeStatus)
		_, _ = w.Write([]byte("success"))
		return
	}
	paidOrder, shouldCallback, err := g.store.MarkPaid(order.OutTradeNo, notification.TradeNo)
	if err != nil {
		g.logger.Printf("mark paid failed order=%s error=%v", order.OutTradeNo, err)
		_, _ = w.Write([]byte("fail"))
		return
	}
	if shouldCallback {
		g.processCallback(paidOrder)
	}
	_, _ = w.Write([]byte("success"))
}

func (g *Gateway) handleAlipayReturn(w http.ResponseWriter, r *http.Request) {
	outTradeNo := r.URL.Query().Get("out_trade_no")
	if outTradeNo != "" {
		if order, ok, err := g.store.Find(outTradeNo); err == nil && ok && order.ReturnURL != "" {
			http.Redirect(w, r, order.ReturnURL, http.StatusFound)
			return
		}
	}
	http.Redirect(w, r, "https://kittyvibe.ai/console/log", http.StatusFound)
}

func (g *Gateway) processCallback(order *Order) {
	ctx, cancel := callbackContext(g.cfg.HTTPTimeout)
	defer cancel()
	err := g.callback.Send(ctx, order)
	if err != nil {
		g.logger.Printf("New API callback failed order=%s error=%v", order.OutTradeNo, err)
		_ = g.store.RecordCallback(order.OutTradeNo, false, err.Error())
		return
	}
	_ = g.store.RecordCallback(order.OutTradeNo, true, "")
}

func firstFormValues(values url.Values) map[string]string {
	params := map[string]string{}
	for key, value := range values {
		if len(value) > 0 {
			params[key] = value[0]
		}
	}
	return params
}

func parseLimitedForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBodyBytes)
	return r.ParseForm()
}

func writeFormError(w http.ResponseWriter, err error, fallback string) {
	if isRequestBodyTooLarge(err) {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, fallback, http.StatusBadRequest)
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func normalizeAmount(raw string) (string, error) {
	amount, err := decimal.NewFromString(raw)
	if err != nil {
		return "", fmt.Errorf("invalid money")
	}
	if !amount.IsPositive() {
		return "", fmt.Errorf("money must be positive")
	}
	if amount.Exponent() < -2 {
		return "", fmt.Errorf("money supports at most two decimal places")
	}
	return amount.StringFixed(2), nil
}

func (g *Gateway) RetryPendingCallbacks(ctx context.Context, maxAttempts int) error {
	orders, err := g.store.RetryableOrders(maxAttempts)
	if err != nil {
		return err
	}
	for _, order := range orders {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			g.processCallback(order)
		}
	}
	return nil
}
