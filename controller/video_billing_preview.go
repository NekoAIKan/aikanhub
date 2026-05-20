package controller

import (
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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
	// HasAudioInput maps to per_second profiles whose with-audio rate
	// differs from the no-audio rate. Ignored by formula profiles.
	HasAudioInput bool `json:"has_audio_input"`
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
	Mode                string  `json:"mode,omitempty"`
	Resolution          string  `json:"resolution,omitempty"`
	Width               int     `json:"width"`
	Height              int     `json:"height"`
	FPS                 int     `json:"fps"`
	DurationSeconds     int     `json:"duration_seconds"`
	HasVideoInput       bool    `json:"has_video_input"`
	HasAudioInput       bool    `json:"has_audio_input,omitempty"`
	Tokens              int     `json:"tokens"`
	UnitPricePerMillion float64 `json:"unit_price_per_million"`
	PricePerSecondUSD   float64 `json:"price_per_second_usd,omitempty"`
	PriceUSD            float64 `json:"price_usd"`
	Quota               int     `json:"quota"`
	QuotaPerUnit        float64 `json:"quota_per_unit"`
}

type studioVideoPricingPreviewRequest struct {
	Model           string                 `json:"model"`
	Resolution      string                 `json:"resolution"`
	Size            string                 `json:"size"`
	Duration        int                    `json:"duration"`
	DurationSeconds int                    `json:"duration_seconds"`
	Seconds         string                 `json:"seconds"`
	BatchCount      int                    `json:"batch_count"`
	BatchCountAlias int                    `json:"batchCount"`
	BatchSize       int                    `json:"batch_size"`
	BatchSizeAlias  int                    `json:"batchSize"`
	GenerateAudio   bool                   `json:"generate_audio"`
	Content         []studioVideoMediaItem `json:"content"`
	References      []studioVideoMediaItem `json:"references"`
	Metadata        map[string]any         `json:"metadata"`
}

type studioVideoMediaItem struct {
	Type string `json:"type"`
}

type studioVideoPricingLineItem struct {
	Label   string `json:"label"`
	Credits int    `json:"credits"`
}

type studioVideoPricingPreviewResponse struct {
	Model           string                       `json:"model"`
	ProfileFound    bool                         `json:"profileFound"`
	Credits         int                          `json:"credits"`
	Currency        string                       `json:"currency"`
	UsageKeyLabel   string                       `json:"usageKeyLabel"`
	Basis           []string                     `json:"basis"`
	LineItems       []studioVideoPricingLineItem `json:"lineItems"`
	Quota           int                          `json:"quota"`
	QuotaPerUnit    float64                      `json:"quotaPerUnit"`
	DurationSeconds int                          `json:"durationSeconds"`
	Resolution      string                       `json:"resolution,omitempty"`
	HasVideoInput   bool                         `json:"hasVideoInput"`
	HasAudioInput   bool                         `json:"hasAudioInput,omitempty"`
	BatchCount      int                          `json:"batchCount"`
}

// StudioVideoPricingPreview powers the Studio Generate button estimate. It is
// advisory only; the relay billing path remains authoritative at submit/settle.
func StudioVideoPricingPreview(c *gin.Context) {
	var req studioVideoPricingPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model required"})
		return
	}

	profile, ok := videobilling.GetProfile(model)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "pricing profile not configured"})
		return
	}

	duration := firstPositiveInt(req.DurationSeconds, req.Duration, intFromString(req.Seconds), intFromAny(req.Metadata["duration"]), intFromAny(req.Metadata["seconds"]), profile.FallbackDurationSeconds)
	if duration <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "duration must be positive"})
		return
	}

	resolution := firstString(strings.TrimSpace(req.Resolution), strings.TrimSpace(req.Size), stringFromAny(req.Metadata["resolution"]), stringFromAny(req.Metadata["size"]))
	resolution = strings.ToLower(resolution)
	hasVideoInput := studioPreviewHasMediaType(req.Content, "video_url") || studioPreviewHasMediaType(req.References, "video_url") || metadataContentHasType(req.Metadata, "video_url")
	hasAudioInput := req.GenerateAudio || boolFromAny(req.Metadata["generate_audio"]) || boolFromAny(req.Metadata["generate_audio_switch"])
	batchCount := clampPositive(firstPositiveInt(req.BatchCount, req.BatchCountAlias, req.BatchSize, req.BatchSizeAlias, intFromAny(req.Metadata["batch_count"])), 1, 100)

	width, height := 0, 0
	if resolution != "" && profile.ResolutionAliases != nil {
		if dim, found := profile.ResolutionAliases[resolution]; found {
			width, height = dim.Width, dim.Height
		}
	}
	if width <= 0 {
		width = profile.FallbackWidth
	}
	if height <= 0 {
		height = profile.FallbackHeight
	}
	fps := profile.FallbackFPS
	if fps <= 0 {
		fps = 24
	}

	input := service.VideoBillingInput{
		OutputSeconds:     duration,
		Width:             width,
		Height:            height,
		FPS:               fps,
		Resolution:        resolution,
		GroupRatio:        studioPreviewGroupRatio(c),
		HasReferenceMedia: hasVideoInput,
		HasAudioInput:     hasAudioInput,
	}
	if hasVideoInput {
		input.InputSeconds = duration
	}

	result := service.CalculateVideoBilling(profile, input, false)
	perItemCredits := quotaToStudioCredits(result.Quota)
	totalCredits := perItemCredits * batchCount
	lineItems := []studioVideoPricingLineItem{{
		Label:   strings.TrimSpace(strings.Join([]string{durationLabel(duration), resolution}, " ")),
		Credits: perItemCredits,
	}}
	if batchCount > 1 {
		lineItems = append(lineItems, studioVideoPricingLineItem{Label: "Batch x" + strconv.Itoa(batchCount), Credits: totalCredits})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": studioVideoPricingPreviewResponse{
		Model:           model,
		ProfileFound:    true,
		Credits:         totalCredits,
		Currency:        "credits",
		UsageKeyLabel:   "Studio",
		Basis:           []string{"duration_seconds", "resolution", "has_video_input", "has_audio_input", "batch_count"},
		LineItems:       lineItems,
		Quota:           result.Quota * batchCount,
		QuotaPerUnit:    common.QuotaPerUnit,
		DurationSeconds: duration,
		Resolution:      result.Resolution,
		HasVideoInput:   hasVideoInput,
		HasAudioInput:   hasAudioInput,
		BatchCount:      batchCount,
	}})
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
		HasAudioInput:   req.HasAudioInput,
		QuotaPerUnit:    common.QuotaPerUnit,
	}
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
		return
	}
	resp.Mode = profile.Mode

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
		HasAudioInput:     req.HasAudioInput,
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
	resp.PricePerSecondUSD = result.PricePerSecondUSD
	// Per_second mode populates RawCost in real USD; formula mode
	// populates it in `tokens × $/M tokens` and needs /1M to convert.
	if profile.Mode == videobilling.ModePerSecond {
		resp.PriceUSD = result.RawCost
	} else {
		resp.PriceUSD = result.RawCost / 1_000_000
	}
	resp.Quota = result.Quota

	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func studioPreviewGroupRatio(c *gin.Context) float64 {
	usingGroup := c.GetString("group")
	if usingGroup == "" {
		usingGroup = c.GetString("user_group")
	}
	groupRatio := ratio_setting.GetGroupRatio(usingGroup)
	if userGroup := c.GetString("user_group"); userGroup != "" && usingGroup != "" {
		if specialRatio, ok := ratio_setting.GetGroupGroupRatio(userGroup, usingGroup); ok {
			groupRatio = specialRatio
		}
	}
	if groupRatio <= 0 {
		return 1
	}
	return groupRatio
}

func quotaToStudioCredits(quota int) int {
	if quota <= 0 {
		return 1
	}
	if common.QuotaPerUnit <= 0 {
		return quota
	}
	credits := int(math.Ceil(float64(quota) / common.QuotaPerUnit))
	if credits < 1 {
		return 1
	}
	return credits
}

func durationLabel(seconds int) string {
	if seconds <= 0 {
		return "Video generation"
	}
	return strconv.Itoa(seconds) + "s"
}

func studioPreviewHasMediaType(items []studioVideoMediaItem, mediaType string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Type), mediaType) {
			return true
		}
	}
	return false
}

func metadataContentHasType(metadata map[string]any, mediaType string) bool {
	raw, ok := metadata["content"].([]any)
	if !ok {
		return false
	}
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(stringFromAny(entry["type"])), mediaType) {
			return true
		}
	}
	return false
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func clampPositive(value int, minValue int, maxValue int) int {
	if value <= 0 {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

func intFromString(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func intFromAny(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		return intFromString(value)
	default:
		return 0
	}
}

func stringFromAny(raw any) string {
	value, _ := raw.(string)
	return strings.TrimSpace(value)
}

func boolFromAny(raw any) bool {
	value, _ := raw.(bool)
	return value
}
