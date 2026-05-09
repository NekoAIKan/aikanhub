package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CallbackSender struct {
	PID    string
	Key    string
	Client *http.Client
}

func NewCallbackSender(cfg Config) *CallbackSender {
	return &CallbackSender{
		PID:    cfg.EPayPID,
		Key:    cfg.EPayKey,
		Client: &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

func (s *CallbackSender) Params(order *Order) map[string]string {
	params := map[string]string{
		"pid":          s.PID,
		"type":         order.PaymentType,
		"out_trade_no": order.OutTradeNo,
		"trade_no":     order.AlipayTradeNo,
		"name":         order.Subject,
		"money":        order.Amount,
		"trade_status": "TRADE_SUCCESS",
	}
	return AddEPaySignature(params, s.Key)
}

func (s *CallbackSender) Send(ctx context.Context, order *Order) error {
	if order.NotifyURL == "" {
		return fmt.Errorf("missing notify_url")
	}
	form := url.Values{}
	for k, v := range s.Params(order) {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, order.NotifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(body)) != "success" {
		return fmt.Errorf("New API callback returned status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("New API callback returned non-2xx status=%d", resp.StatusCode)
	}
	return nil
}

func callbackContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
