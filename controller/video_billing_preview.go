package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"

	"github.com/gin-gonic/gin"
)

// videoBillingPreviewFixture is a canonical request shape used to demonstrate
// what a profile resolves to. The shapes themselves are vendor-agnostic —
// they exercise the four lookup branches (with-video × resolution) plus the
// MinTokensWithVideo floor.
type videoBillingPreviewFixture struct {
	Label string
	Input service.VideoBillingInput
}

// canonicalPreviewFixtures returns the request shapes the admin preview
// renders. Kept server-side so the wire shape and the live billing path
// share one source of truth.
func canonicalPreviewFixtures() []videoBillingPreviewFixture {
	return []videoBillingPreviewFixture{
		{
			Label: "720p / 5s text",
			Input: service.VideoBillingInput{
				OutputSeconds: 5, Width: 1280, Height: 720, FPS: 24,
				Resolution: "720p", GroupRatio: 1,
			},
		},
		{
			Label: "1080p / 5s text",
			Input: service.VideoBillingInput{
				OutputSeconds: 5, Width: 1920, Height: 1080, FPS: 24,
				Resolution: "1080p", GroupRatio: 1,
			},
		},
		{
			Label: "720p / 5s+5s with reference",
			Input: service.VideoBillingInput{
				InputSeconds: 5, OutputSeconds: 5, Width: 1280, Height: 720, FPS: 24,
				Resolution: "720p", GroupRatio: 1, HasReferenceMedia: true,
			},
		},
		{
			Label: "1080p / 5s+5s with reference",
			Input: service.VideoBillingInput{
				InputSeconds: 5, OutputSeconds: 5, Width: 1920, Height: 1080, FPS: 24,
				Resolution: "1080p", GroupRatio: 1, HasReferenceMedia: true,
			},
		},
		{
			Label: "720p / 1s+2s with reference (floor)",
			Input: service.VideoBillingInput{
				InputSeconds: 1, OutputSeconds: 2, Width: 1280, Height: 720, FPS: 24,
				Resolution: "720p", GroupRatio: 1, HasReferenceMedia: true,
			},
		},
	}
}

type videoBillingPreviewRequest struct {
	Model string `json:"model"`
}

type videoBillingPreviewCase struct {
	Label               string  `json:"label"`
	Tokens              int     `json:"tokens"`
	UnitPricePerMillion float64 `json:"unit_price_per_million"`
	RawCostUSD          float64 `json:"raw_cost_usd"`
	Quota               int     `json:"quota"`
	HasReferenceMedia   bool    `json:"has_reference_media"`
	Resolution          string  `json:"resolution,omitempty"`
	Basis               string  `json:"basis"`
}

type videoBillingPreviewResponse struct {
	Model         string                    `json:"model"`
	ProfileFound  bool                      `json:"profile_found"`
	QuotaPerUnit  float64                   `json:"quota_per_unit"`
	Cases         []videoBillingPreviewCase `json:"cases"`
	AvailableKeys []string                  `json:"available_models"`
}

// PreviewVideoBilling resolves canonical fixture request shapes against the
// admin-saved video billing profile for the requested model and returns the
// computed tokens / unit price / quota for each — i.e. what the customer
// actually pays for a typical request, derived from the same code path the
// runtime uses (service.CalculateVideoBilling).
func PreviewVideoBilling(c *gin.Context) {
	var req videoBillingPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	available := videobilling.ListConfiguredModels()
	model := strings.TrimSpace(req.Model)
	resp := videoBillingPreviewResponse{
		Model:         model,
		QuotaPerUnit:  common.QuotaPerUnit,
		AvailableKeys: available,
	}

	if model == "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
		return
	}

	profile, ok := videobilling.GetProfile(model)
	resp.ProfileFound = ok
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
		return
	}

	for _, fixture := range canonicalPreviewFixtures() {
		result := service.CalculateVideoBilling(profile, fixture.Input, false)
		resp.Cases = append(resp.Cases, videoBillingPreviewCase{
			Label:               fixture.Label,
			Tokens:              result.Tokens,
			UnitPricePerMillion: result.RetailUnitPrice,
			RawCostUSD:          result.RawCost / 1_000_000,
			Quota:               result.Quota,
			HasReferenceMedia:   fixture.Input.HasReferenceMedia,
			Resolution:          fixture.Input.Resolution,
			Basis:               result.Basis,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}
