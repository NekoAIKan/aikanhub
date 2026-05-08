package main

import (
	"crypto/rsa"
	"fmt"
	"html"
	"net/url"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const alipayPagePayMethod = "alipay.trade.page.pay"

type AlipayClient struct {
	cfg        Config
	privateKey *rsa.PrivateKey
	now        func() time.Time
}

func NewAlipayClient(cfg Config) *AlipayClient {
	return &AlipayClient{cfg: cfg, now: time.Now}
}

func NewAlipayClientWithKey(cfg Config, privateKey *rsa.PrivateKey) *AlipayClient {
	return &AlipayClient{cfg: cfg, privateKey: privateKey, now: time.Now}
}

func (c *AlipayClient) PagePayParams(order *Order) (map[string]string, error) {
	bizContent, err := common.Marshal(map[string]string{
		"out_trade_no": order.OutTradeNo,
		"product_code": "FAST_INSTANT_TRADE_PAY",
		"total_amount": order.Amount,
		"subject":      order.Subject,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal Alipay biz_content: %w", err)
	}

	params := map[string]string{
		"app_id":      c.cfg.AlipayAppID,
		"method":      alipayPagePayMethod,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   c.now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  stringsJoinURL(c.cfg.PublicBaseURL, "/alipay/notify"),
		"return_url":  stringsJoinURL(c.cfg.PublicBaseURL, "/alipay/return"),
		"biz_content": string(bizContent),
	}

	var signature string
	if c.privateKey != nil {
		signature, err = signAlipayParamsWithKey(params, c.privateKey)
	} else {
		signature, err = signAlipayParams(params, c.cfg.AlipayAppPrivateKey)
	}
	if err != nil {
		return nil, err
	}
	params["sign"] = signature
	return params, nil
}

func (c *AlipayClient) PagePayForm(order *Order) (string, error) {
	params, err := c.PagePayParams(order)
	if err != nil {
		return "", err
	}
	return renderAutoSubmitForm(c.cfg.AlipayGateway, params), nil
}

func renderAutoSubmitForm(action string, params map[string]string) string {
	form := "<!doctype html><html><head><meta charset=\"utf-8\"><title>Redirecting to Alipay</title></head><body>"
	form += "<form id=\"alipaysubmit\" name=\"alipaysubmit\" action=\"" + html.EscapeString(action) + "\" method=\"POST\">"
	for key, value := range params {
		form += "<input type=\"hidden\" name=\"" + html.EscapeString(key) + "\" value=\"" + html.EscapeString(value) + "\"/>"
	}
	form += "</form><script>document.getElementById('alipaysubmit').submit();</script></body></html>"
	return form
}

func stringsJoinURL(base string, path string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return base + path
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
