package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddr            = ":3001"
	defaultAlipayGateway         = "https://openapi.alipay.com/gateway.do"
	defaultAlipaySandboxGateway  = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
	defaultOrderStorePath        = "epay-orders.json"
	defaultAllowedNotifyHost     = "kittyvibe.ai"
	defaultCallbackRetryInterval = 30 * time.Second
	defaultHTTPTimeout           = 10 * time.Second
)

type Config struct {
	ListenAddr            string
	PublicBaseURL         string
	EPayPID               string
	EPayKey               string
	AllowedNotifyHosts    map[string]struct{}
	AlipayAppID           string
	AlipayAppPrivateKey   string
	AlipayPublicKey       string
	AlipaySellerID        string
	AlipaySandbox         bool
	AlipayGateway         string
	OrderStorePath        string
	HTTPTimeout           time.Duration
	CallbackRetryInterval time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	sandbox := parseBoolEnv("ALIPAY_SANDBOX", false)
	alipayGateway := defaultAlipayGateway
	if sandbox {
		alipayGateway = defaultAlipaySandboxGateway
	}

	cfg := Config{
		ListenAddr:            getenvDefault("LISTEN_ADDR", defaultListenAddr),
		PublicBaseURL:         strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/"),
		EPayPID:               strings.TrimSpace(os.Getenv("EPAY_PID")),
		EPayKey:               strings.TrimSpace(os.Getenv("EPAY_KEY")),
		AllowedNotifyHosts:    parseHostSet(getenvDefault("ALLOWED_NOTIFY_HOSTS", defaultAllowedNotifyHost)),
		AlipayAppID:           strings.TrimSpace(os.Getenv("ALIPAY_APP_ID")),
		AlipayAppPrivateKey:   strings.TrimSpace(os.Getenv("ALIPAY_APP_PRIVATE_KEY")),
		AlipayPublicKey:       strings.TrimSpace(os.Getenv("ALIPAY_PUBLIC_KEY")),
		AlipaySellerID:        strings.TrimSpace(os.Getenv("ALIPAY_SELLER_ID")),
		AlipaySandbox:         sandbox,
		AlipayGateway:         strings.TrimRight(getenvDefault("ALIPAY_GATEWAY", alipayGateway), "/"),
		OrderStorePath:        getenvDefault("ORDER_STORE_PATH", defaultOrderStorePath),
		HTTPTimeout:           parseDurationEnv("HTTP_TIMEOUT", defaultHTTPTimeout),
		CallbackRetryInterval: parseDurationEnv("CALLBACK_RETRY_INTERVAL", defaultCallbackRetryInterval),
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.PublicBaseURL) == "" {
		return fmt.Errorf("PUBLIC_BASE_URL is required")
	}
	if parsed, err := url.Parse(c.PublicBaseURL); err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("PUBLIC_BASE_URL must be an https URL")
	}
	if strings.TrimSpace(c.EPayPID) == "" {
		return fmt.Errorf("EPAY_PID is required")
	}
	if strings.TrimSpace(c.EPayKey) == "" {
		return fmt.Errorf("EPAY_KEY is required")
	}
	if strings.TrimSpace(c.AlipayAppID) == "" {
		return fmt.Errorf("ALIPAY_APP_ID is required")
	}
	if strings.TrimSpace(c.AlipayAppPrivateKey) == "" {
		return fmt.Errorf("ALIPAY_APP_PRIVATE_KEY is required")
	}
	if strings.TrimSpace(c.AlipayPublicKey) == "" {
		return fmt.Errorf("ALIPAY_PUBLIC_KEY is required")
	}
	if !c.AlipaySandbox && strings.TrimSpace(c.AlipaySellerID) == "" {
		return fmt.Errorf("ALIPAY_SELLER_ID is required unless ALIPAY_SANDBOX=true")
	}
	if parsed, err := url.Parse(c.AlipayGateway); err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("ALIPAY_GATEWAY must be an https URL")
	}
	if len(c.AllowedNotifyHosts) == 0 {
		return fmt.Errorf("ALLOWED_NOTIFY_HOSTS must include at least one host")
	}
	return nil
}

func (c Config) IsAllowedCallbackURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid callback URL")
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("callback URL must use https")
	}
	host := strings.ToLower(parsed.Hostname())
	if _, ok := c.AllowedNotifyHosts[host]; !ok {
		return nil, fmt.Errorf("callback host %q is not allowed", host)
	}
	return parsed, nil
}

func getenvDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseHostSet(raw string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		host := strings.ToLower(strings.TrimSpace(part))
		if host == "" {
			continue
		}
		result[host] = struct{}{}
	}
	return result
}

func parseBoolEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
