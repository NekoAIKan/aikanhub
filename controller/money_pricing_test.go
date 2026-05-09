package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMoneyPricingControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(&model.FxRate{}, &model.ChannelModelCost{}, &model.RetailPricingPolicy{}))
	return db
}

func performMoneyPricingControllerRequest(t *testing.T, method string, path string, body string, params gin.Params, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = params
	handler(ctx)
	return recorder
}

func decodeMoneyPricingControllerResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestMoneyPricingAdminCreatesUpdatesDeletesFxRate(t *testing.T) {
	setupMoneyPricingControllerTestDB(t)

	create := performMoneyPricingControllerRequest(t, http.MethodPost, "/api/money_pricing/fx_rates", `{
		"id":"fx-cny-usd",
		"base_currency":"cny",
		"quote_currency":"usd",
		"rate_micros":137000,
		"buffer_bps":500,
		"source":"manual",
		"effective_at":1000
	}`, nil, CreateMoneyPricingFxRate)
	require.True(t, decodeMoneyPricingControllerResponse(t, create)["success"].(bool))

	update := performMoneyPricingControllerRequest(t, http.MethodPut, "/api/money_pricing/fx_rates/fx-cny-usd", `{
		"base_currency":"CNY",
		"quote_currency":"USD",
		"rate_micros":140000,
		"buffer_bps":300,
		"source":"manual",
		"effective_at":2000
	}`, gin.Params{{Key: "id", Value: "fx-cny-usd"}}, UpdateMoneyPricingFxRate)
	require.True(t, decodeMoneyPricingControllerResponse(t, update)["success"].(bool))
	rate, err := model.GetFxRateById("fx-cny-usd")
	require.NoError(t, err)
	require.Equal(t, int64(140000), rate.RateMicros)
	require.Equal(t, "USD", rate.QuoteCurrency)

	remove := performMoneyPricingControllerRequest(t, http.MethodDelete, "/api/money_pricing/fx_rates/fx-cny-usd", "", gin.Params{{Key: "id", Value: "fx-cny-usd"}}, DeleteMoneyPricingFxRate)
	require.True(t, decodeMoneyPricingControllerResponse(t, remove)["success"].(bool))
}

func TestMoneyPricingAdminRejectsInvalidChannelModelCostProfile(t *testing.T) {
	setupMoneyPricingControllerTestDB(t)

	recorder := performMoneyPricingControllerRequest(t, http.MethodPost, "/api/money_pricing/channel_model_costs", `{
		"channel_id": 7,
		"upstream_model": "upstream",
		"endpoint_type": "openai",
		"billing_rule_json": "{\"schema_version\":",
		"source": "manual",
		"enabled": true
	}`, nil, CreateMoneyPricingChannelModelCost)

	response := decodeMoneyPricingControllerResponse(t, recorder)
	require.False(t, response["success"].(bool))
	require.Contains(t, response["message"].(string), "money pricing profile")
}

func TestMoneyPricingAdminCreatesCostPlusRetailPolicyWithEmptyProfile(t *testing.T) {
	setupMoneyPricingControllerTestDB(t)

	recorder := performMoneyPricingControllerRequest(t, http.MethodPost, "/api/money_pricing/retail_policies", `{
		"public_model": "public-chat",
		"group": "default",
		"endpoint_type": "openai",
		"pricing_mode": "cost_plus",
		"currency": "USD",
		"enabled": true
	}`, nil, CreateMoneyPricingRetailPolicy)

	response := decodeMoneyPricingControllerResponse(t, recorder)
	require.True(t, response["success"].(bool))
	policies, total, err := model.ListRetailPricingPolicies(0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, model.PricingModeCostPlus, policies[0].PricingMode)
}

func TestMoneyPricingQuotePreviewReturnsRetailAmount(t *testing.T) {
	setupMoneyPricingControllerTestDB(t)

	recorder := performMoneyPricingControllerRequest(t, http.MethodPost, "/api/money_pricing/quote_preview", `{
		"policy": {
			"public_model": "public-chat",
			"group": "default",
			"endpoint_type": "openai",
			"pricing_mode": "fixed_rule",
			"currency": "USD",
			"billing_rule_json": "{\"schema_version\":1,\"profile_type\":\"money_usage_pricing\",\"currency\":\"USD\",\"rates\":{\"input_token\":{\"amount_micros\":2000000,\"basis\":\"per_million_units\",\"currency\":\"USD\"}}}"
		},
		"features": {
			"InputTokens": 1000000
		},
		"settlement_currency": "USD"
	}`, nil, PreviewMoneyPricingQuote)

	response := decodeMoneyPricingControllerResponse(t, recorder)
	require.True(t, response["success"].(bool))
	data := response["data"].(map[string]any)
	require.Equal(t, float64(2_000_000), data["RetailAmountMicros"])
}
