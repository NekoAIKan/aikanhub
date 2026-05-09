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

type calculateVideoBillingRequest struct {
	Model           string `json:"model"`
	Resolution      string `json:"resolution"`
	DurationSeconds int    `json:"duration_seconds"`
	HasVideoInput   bool   `json:"has_video_input"`
	// Width/Height are optional; if omitted the resolution alias from
	// the profile is used. They exist so callers integrating against
	// vendors that publish e.g. 1280×704 can still query precisely.
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
}

type calculateVideoBillingResponse struct {
	Model               string  `json:"model"`
	ProfileFound        bool    `json:"profile_found"`
	Resolution          string  `json:"resolution,omitempty"`
	Width               int     `json:"width"`
	Height              int     `json:"height"`
	FPS                 int     `json:"fps"`
	DurationSeconds     int     `json:"duration_seconds"`
	HasVideoInput       bool    `json:"has_video_input"`
	Tokens              int     `json:"tokens"`
	UnitPricePerMillion float64 `json:"unit_price_per_million"`
	PriceUSD            float64 `json:"price_usd"`
	Quota               int     `json:"quota"`
	QuotaPerUnit        float64 `json:"quota_per_unit"`
}

// CalculateVideoBillingPrice powers the live calculator on the /pricing
// per-model details page. It runs the same code path as runtime billing
// (service.CalculateVideoBilling) so the figure shown to the customer
// matches what they'll be charged. Public — intentionally no auth — so
// the page works for visitors evaluating pricing before signing up.
//
// What this endpoint deliberately does NOT return:
//   - upstream_unit_cost / markup_percent (operator margin, internal)
//   - basis (formula vs upstream_usage — implementation detail)
//   - reference_conservative_multiplier output (pre-charge inflation,
//     refunded on settle; would mislead the customer about true cost)
func CalculateVideoBillingPrice(c *gin.Context) {
	var req calculateVideoBillingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model required"})
		return
	}
	if req.DurationSeconds <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "duration_seconds must be positive"})
		return
	}

	profile, ok := videobilling.GetProfile(model)
	resp := calculateVideoBillingResponse{
		Model:           model,
		ProfileFound:    ok,
		Resolution:      strings.TrimSpace(strings.ToLower(req.Resolution)),
		DurationSeconds: req.DurationSeconds,
		HasVideoInput:   req.HasVideoInput,
		QuotaPerUnit:    common.QuotaPerUnit,
	}
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
		return
	}

	// Resolve dimensions: explicit width/height > resolution alias from
	// profile > profile fallback. The runtime path normalises the same
	// way; we mirror it so the calculator never disagrees with billing.
	width, height := req.Width, req.Height
	if width <= 0 || height <= 0 {
		if alias := resp.Resolution; alias != "" && profile.ResolutionAliases != nil {
			if dim, found := profile.ResolutionAliases[alias]; found && dim.Width > 0 && dim.Height > 0 {
				width, height = dim.Width, dim.Height
			}
		}
	}
	if width <= 0 {
		width = profile.FallbackWidth
	}
	if height <= 0 {
		height = profile.FallbackHeight
	}
	fps := req.FPS
	if fps <= 0 {
		fps = profile.FallbackFPS
	}
	if fps <= 0 {
		fps = 24
	}

	input := service.VideoBillingInput{
		OutputSeconds:     req.DurationSeconds,
		Width:             width,
		Height:            height,
		FPS:               fps,
		Resolution:        resp.Resolution,
		GroupRatio:        1, // group ratio is private to the caller's account
		HasReferenceMedia: req.HasVideoInput,
	}
	if req.HasVideoInput {
		// Mirrors the request-side default in
		// relay/channel/task/doubao/billing.ExtractRequestBillingInput:
		// when a reference video is present and the caller didn't say
		// otherwise, assume it's the same length as the requested output.
		input.InputSeconds = req.DurationSeconds
	}

	result := service.CalculateVideoBilling(profile, input, false)
	resp.Width = result.Width
	resp.Height = result.Height
	resp.FPS = result.FPS
	resp.Tokens = result.Tokens
	resp.UnitPricePerMillion = result.RetailUnitPrice
	resp.PriceUSD = result.RawCost / 1_000_000
	resp.Quota = result.Quota

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}
