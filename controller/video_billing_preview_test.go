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
