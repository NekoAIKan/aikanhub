package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPreviewVideoBilling_EmptyModelReturnsAvailableList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"video_billing_setting.profiles": `{
			"alpha-model": {"mode":"formula","unit_price":1,"fallback_fps":24,"fallback_width":1280,"fallback_height":720,"fallback_duration_seconds":5},
			"beta-model":  {"mode":"formula","unit_price":2,"fallback_fps":24,"fallback_width":1280,"fallback_height":720,"fallback_duration_seconds":5}
		}`,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/option/video_billing/preview",
		bytes.NewBufferString(`{"model": ""}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	PreviewVideoBilling(ctx)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Model           string   `json:"model"`
			ProfileFound    bool     `json:"profile_found"`
			AvailableModels []string `json:"available_models"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.False(t, resp.Data.ProfileFound)
	require.Empty(t, resp.Data.Model)
	got := resp.Data.AvailableModels
	sort.Strings(got)
	require.Equal(t, []string{"alpha-model", "beta-model"}, got)
}

func TestPreviewVideoBilling_UnknownModelReturnsNotFoundButShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"video_billing_setting.profiles": `{}`,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/option/video_billing/preview",
		bytes.NewBufferString(`{"model": "ghost"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	PreviewVideoBilling(ctx)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Model        string `json:"model"`
			ProfileFound bool   `json:"profile_found"`
			Cases        []any  `json:"cases"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Equal(t, "ghost", resp.Data.Model)
	require.False(t, resp.Data.ProfileFound)
	require.Empty(t, resp.Data.Cases)
}

// TestPreviewVideoBilling_SeedanceMatrix loads the same fixture profile that
// service/video_billing_test.go::TestSeedanceFixturePricingMatrix uses and
// asserts the controller returns the documented quota for each canonical
// request shape — i.e. the wire shape and the runtime billing path agree.
func TestPreviewVideoBilling_SeedanceMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.upstream_cost_per_million_tokens": "0",
		"profit_setting.default_markup_percent":           "0",
		"profit_setting.apply_to_default_video_profiles":  "false",
		"video_billing_setting.profiles": `{
			"doubao-seedance-2-0-260128": {
				"mode": "formula",
				"unit_price": 7.89,
				"unit_price_with_video": 4.80,
				"unit_price_by_resolution": {"480p": 7.89, "720p": 7.89, "1080p": 8.74},
				"unit_price_with_video_by_resolution": {"480p": 4.80, "720p": 4.80, "1080p": 5.31},
				"min_tokens_with_video": 108000,
				"fallback_fps": 24,
				"fallback_width": 1280,
				"fallback_height": 720,
				"fallback_duration_seconds": 5,
				"use_upstream_usage": true,
				"conservative_multiplier": 1.25,
				"reference_conservative_multiplier": 2.0,
				"draft_multiplier": 0.5
			}
		}`,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/option/video_billing/preview",
		bytes.NewBufferString(`{"model": "doubao-seedance-2-0-260128"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	PreviewVideoBilling(ctx)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Model        string `json:"model"`
			ProfileFound bool   `json:"profile_found"`
			QuotaPerUnit float64 `json:"quota_per_unit"`
			Cases        []struct {
				Label               string  `json:"label"`
				Tokens              int     `json:"tokens"`
				UnitPricePerMillion float64 `json:"unit_price_per_million"`
				Quota               int     `json:"quota"`
				HasReferenceMedia   bool    `json:"has_reference_media"`
				Resolution          string  `json:"resolution,omitempty"`
			} `json:"cases"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.True(t, resp.Data.ProfileFound)
	require.Equal(t, common.QuotaPerUnit, resp.Data.QuotaPerUnit)

	byLabel := map[string]struct {
		Tokens int
		Price  float64
		Quota  int
	}{}
	for _, c := range resp.Data.Cases {
		byLabel[c.Label] = struct {
			Tokens int
			Price  float64
			Quota  int
		}{c.Tokens, c.UnitPricePerMillion, c.Quota}
	}

	// Numbers come from the same matrix asserted in service/video_billing_test.go;
	// any drift between the controller wire format and the billing engine flips
	// these assertions before reaching production.
	require.Equal(t, 108000, byLabel["720p / 5s text"].Tokens)
	require.Equal(t, 7.89, byLabel["720p / 5s text"].Price)
	require.Equal(t, 426060, byLabel["720p / 5s text"].Quota)

	require.Equal(t, 243000, byLabel["1080p / 5s text"].Tokens)
	require.Equal(t, 8.74, byLabel["1080p / 5s text"].Price)
	require.Equal(t, 1061910, byLabel["1080p / 5s text"].Quota)

	require.Equal(t, 216000, byLabel["720p / 5s+5s with reference"].Tokens)
	require.Equal(t, 4.80, byLabel["720p / 5s+5s with reference"].Price)
	require.Equal(t, 518400, byLabel["720p / 5s+5s with reference"].Quota)

	require.Equal(t, 486000, byLabel["1080p / 5s+5s with reference"].Tokens)
	require.Equal(t, 5.31, byLabel["1080p / 5s+5s with reference"].Price)
	require.Equal(t, 1290330, byLabel["1080p / 5s+5s with reference"].Quota)

	require.Equal(t, 108000, byLabel["720p / 1s+2s with reference (floor)"].Tokens)
	require.Equal(t, 4.80, byLabel["720p / 1s+2s with reference (floor)"].Price)
	require.Equal(t, 259200, byLabel["720p / 1s+2s with reference (floor)"].Quota)
}

// Live calculator endpoint (used by the per-model details page).
func TestCalculateVideoBillingPrice_TextRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.upstream_cost_per_million_tokens": "0",
		"profit_setting.default_markup_percent":           "0",
		"profit_setting.apply_to_default_video_profiles":  "false",
		"video_billing_setting.profiles": `{
			"doubao-seedance-2-0-260128": {
				"mode": "formula",
				"unit_price": 7.89,
				"unit_price_with_video": 4.80,
				"unit_price_by_resolution": {"720p": 7.89, "1080p": 8.74},
				"unit_price_with_video_by_resolution": {"720p": 4.80, "1080p": 5.31},
				"min_tokens_with_video": 108000,
				"fallback_fps": 24,
				"fallback_width": 1280,
				"fallback_height": 720,
				"fallback_duration_seconds": 5,
				"use_upstream_usage": true,
				"conservative_multiplier": 1.25,
				"reference_conservative_multiplier": 2.0,
				"resolution_aliases": {
					"720p":  {"width": 1280, "height": 720},
					"1080p": {"width": 1920, "height": 1080}
				}
			}
		}`,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/pricing/calculate",
		bytes.NewBufferString(`{"model":"doubao-seedance-2-0-260128","resolution":"720p","duration_seconds":5,"has_video_input":false}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CalculateVideoBillingPrice(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ProfileFound        bool    `json:"profile_found"`
			Width               int     `json:"width"`
			Height              int     `json:"height"`
			FPS                 int     `json:"fps"`
			Tokens              int     `json:"tokens"`
			UnitPricePerMillion float64 `json:"unit_price_per_million"`
			PriceUSD            float64 `json:"price_usd"`
			Quota               int     `json:"quota"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.True(t, resp.Data.ProfileFound)
	require.Equal(t, 1280, resp.Data.Width)
	require.Equal(t, 720, resp.Data.Height)
	require.Equal(t, 24, resp.Data.FPS)
	require.Equal(t, 108000, resp.Data.Tokens)
	require.Equal(t, 7.89, resp.Data.UnitPricePerMillion)
	require.InDelta(t, 0.85212, resp.Data.PriceUSD, 1e-6)
	require.Equal(t, 426060, resp.Data.Quota)
}

func TestCalculateVideoBillingPrice_WithVideoHitsFloor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.upstream_cost_per_million_tokens": "0",
		"profit_setting.default_markup_percent":           "0",
		"profit_setting.apply_to_default_video_profiles":  "false",
		"video_billing_setting.profiles": `{
			"doubao-seedance-2-0-260128": {
				"mode": "formula",
				"unit_price_with_video": 4.80,
				"min_tokens_with_video": 108000,
				"fallback_fps": 24,
				"fallback_width": 1280,
				"fallback_height": 720,
				"fallback_duration_seconds": 5,
				"use_upstream_usage": true,
				"resolution_aliases": {"720p": {"width": 1280, "height": 720}}
			}
		}`,
	}))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	// Very short request that would otherwise produce ~64_800 tokens —
	// the floor should pull it up to 108_000.
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/pricing/calculate",
		bytes.NewBufferString(`{"model":"doubao-seedance-2-0-260128","resolution":"720p","duration_seconds":2,"has_video_input":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	CalculateVideoBillingPrice(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data struct {
			Tokens int `json:"tokens"`
			Quota  int `json:"quota"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 108000, resp.Data.Tokens)
	require.Equal(t, 259200, resp.Data.Quota)
}

func TestCalculateVideoBillingPrice_RejectsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing model", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/pricing/calculate",
			bytes.NewBufferString(`{"resolution":"720p","duration_seconds":5}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		CalculateVideoBillingPrice(ctx)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("non-positive duration", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/pricing/calculate",
			bytes.NewBufferString(`{"model":"x","resolution":"720p","duration_seconds":0}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		CalculateVideoBillingPrice(ctx)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("per_second profile returns rate and zero tokens", func(t *testing.T) {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
			"profit_setting.upstream_cost_per_million_tokens": "0",
			"profit_setting.default_markup_percent":           "0",
			"profit_setting.apply_to_default_video_profiles":  "false",
			"video_billing_setting.profiles": `{
				"pixverse-c1": {
					"mode": "per_second",
					"price_per_second_by_resolution": {"540p": 0.048, "720p": 0.06, "1080p": 0.114},
					"price_per_second_with_audio_by_resolution": {"540p": 0.06, "720p": 0.078, "1080p": 0.144},
					"fallback_fps": 24,
					"fallback_width": 960,
					"fallback_height": 540,
					"fallback_duration_seconds": 5,
					"resolution_aliases": {"540p":{"width":960,"height":540},"720p":{"width":1280,"height":720},"1080p":{"width":1920,"height":1080}}
				}
			}`,
		}))
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/pricing/calculate",
			bytes.NewBufferString(`{"model":"pixverse-c1","resolution":"720p","duration_seconds":5,"has_audio_input":true}`))
		ctx.Request.Header.Set("Content-Type", "application/json")

		CalculateVideoBillingPrice(ctx)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp struct {
			Data struct {
				Mode              string  `json:"mode"`
				Tokens            int     `json:"tokens"`
				PricePerSecondUSD float64 `json:"price_per_second_usd"`
				PriceUSD          float64 `json:"price_usd"`
				Quota             int     `json:"quota"`
			} `json:"data"`
		}
		require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, "per_second", resp.Data.Mode)
		require.Equal(t, 0, resp.Data.Tokens)
		require.InDelta(t, 0.078, resp.Data.PricePerSecondUSD, 1e-9)
		require.InDelta(t, 0.39, resp.Data.PriceUSD, 1e-6) // 5 × 0.078
		require.Equal(t, int(0.39*common.QuotaPerUnit), resp.Data.Quota)
	})

	t.Run("calculator does not leak upstream cost or markup", func(t *testing.T) {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
			"profit_setting.upstream_cost_per_million_tokens": "0.5",
			"profit_setting.default_markup_percent":           "100",
			"profit_setting.apply_to_default_video_profiles":  "true",
			"video_billing_setting.profiles": `{
				"x": {"mode":"formula","unit_price":2,"fallback_fps":24,"fallback_width":1280,"fallback_height":720,"fallback_duration_seconds":5,"use_upstream_usage":true}
			}`,
		}))
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/pricing/calculate",
			bytes.NewBufferString(`{"model":"x","resolution":"720p","duration_seconds":5,"has_video_input":false}`))
		ctx.Request.Header.Set("Content-Type", "application/json")
		CalculateVideoBillingPrice(ctx)
		body := rec.Body.String()
		require.NotContains(t, body, "upstream_unit_cost")
		require.NotContains(t, body, "markup_percent")
		require.NotContains(t, body, "upstream_cost_per_million")
	})
}
