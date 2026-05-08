package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidateRequiresHTTPSPublicBaseURL(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.PublicBaseURL = "http://pay.kittyvibe.ai"

	require.ErrorContains(t, cfg.Validate(), "PUBLIC_BASE_URL")
}

func TestConfigValidateRequiresSellerIDInProduction(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.AlipaySellerID = ""
	cfg.AlipaySandbox = false

	require.ErrorContains(t, cfg.Validate(), "ALIPAY_SELLER_ID")
}

func TestConfigValidateAllowsMissingSellerIDInSandbox(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.AlipaySellerID = ""
	cfg.AlipaySandbox = true

	require.NoError(t, cfg.Validate())
}

func TestConfigValidateRequiresDatabaseURLForPostgresStore(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.OrderStoreType = OrderStoreTypePostgres
	cfg.DatabaseURL = ""

	require.ErrorContains(t, cfg.Validate(), "DATABASE_URL")
}

func TestConfigValidateTreatsNeonAsPostgresStore(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.OrderStoreType = OrderStoreTypeNeon
	cfg.DatabaseURL = ""

	require.ErrorContains(t, cfg.Validate(), "DATABASE_URL")
}

func TestConfigValidateAcceptsFileOrderStoreWithoutDatabaseURL(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.OrderStoreType = OrderStoreTypeFile
	cfg.DatabaseURL = ""

	require.NoError(t, cfg.Validate())
}

func TestConfigAllowsOnlyConfiguredCallbackHosts(t *testing.T) {
	cfg := validTestConfig(t)

	_, err := cfg.IsAllowedCallbackURL("https://kittyvibe.ai/api/user/epay/notify")
	require.NoError(t, err)

	_, err = cfg.IsAllowedCallbackURL("https://evil.example/api/user/epay/notify")
	require.ErrorContains(t, err, "not allowed")

	_, err = cfg.IsAllowedCallbackURL("http://kittyvibe.ai/api/user/epay/notify")
	require.ErrorContains(t, err, "https")
}
