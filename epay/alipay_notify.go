package main

import (
	"crypto/rsa"
	"fmt"
	"net/url"
)

type AlipayNotification struct {
	AppID       string
	SellerID    string
	OutTradeNo  string
	TradeNo     string
	TradeStatus string
	TotalAmount string
	Subject     string
}

func VerifyAlipayNotification(values url.Values, cfg Config) (*AlipayNotification, error) {
	publicKey, err := parseRSAPublicKey(cfg.AlipayPublicKey)
	if err != nil {
		return nil, err
	}
	return VerifyAlipayNotificationWithKey(values, cfg, publicKey)
}

func VerifyAlipayNotificationWithKey(values url.Values, cfg Config, publicKey *rsa.PublicKey) (*AlipayNotification, error) {
	if values.Get("sign_type") != "RSA2" {
		return nil, fmt.Errorf("unsupported Alipay sign_type")
	}
	if err := verifyAlipayParamsWithKey(values, publicKey); err != nil {
		return nil, err
	}
	notification := &AlipayNotification{
		AppID:       values.Get("app_id"),
		SellerID:    values.Get("seller_id"),
		OutTradeNo:  values.Get("out_trade_no"),
		TradeNo:     values.Get("trade_no"),
		TradeStatus: values.Get("trade_status"),
		TotalAmount: values.Get("total_amount"),
		Subject:     values.Get("subject"),
	}
	if notification.AppID != cfg.AlipayAppID {
		return nil, fmt.Errorf("Alipay app_id mismatch")
	}
	if cfg.AlipaySellerID != "" && notification.SellerID != cfg.AlipaySellerID {
		return nil, fmt.Errorf("Alipay seller_id mismatch")
	}
	if notification.OutTradeNo == "" {
		return nil, fmt.Errorf("missing Alipay out_trade_no")
	}
	if notification.TotalAmount == "" {
		return nil, fmt.Errorf("missing Alipay total_amount")
	}
	return notification, nil
}

func (n *AlipayNotification) IsPaid() bool {
	return n.TradeStatus == "TRADE_SUCCESS" || n.TradeStatus == "TRADE_FINISHED"
}
