package pixverse

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
)

// ExtractRequestBillingInput resolves a Pixverse task submit request into
// the shape `service.CalculateVideoBilling` consumes. Pixverse C1/V6 bill
// per-second × resolution × ±audio, so the fields we care about are:
//
//	OutputSeconds   – `duration` (top level) / `metadata.duration`
//	Resolution      – `quality` (top level) / `metadata.quality` / `req.Size`
//	HasAudioInput   – `metadata.generate_audio_switch`
//	HasReferenceMedia – never set; with-video pricing is a doubao concept
//
// Width/Height are derived from the resolution alias via the profile's
// alias map; if the alias is unknown they fall back to profile defaults.
// We do NOT propagate FPS — per_second mode does not use it for billing.
func ExtractRequestBillingInput(req relaycommon.TaskSubmitReq, profile videobilling.VideoBillingProfile, groupRatio float64) service.VideoBillingInput {
	duration := firstPositiveInt(
		intFromMap(req.Metadata, "duration"),
		req.Duration,
		atoi(req.Seconds),
		intFromMap(req.Metadata, "seconds"),
		profile.FallbackDurationSeconds,
	)

	resolutionRaw := firstString(
		stringFromMap(req.Metadata, "quality"),
		stringFromMap(req.Metadata, "resolution"),
		req.Resolution,
		req.Size,
		stringFromMap(req.Metadata, "size"),
	)
	resolution := normalizeResolutionAlias(resolutionRaw)
	width, height := resolveDimensions(profile, resolution)

	hasAudio := boolFromMap(req.Metadata, "generate_audio_switch")
	if !hasAudio {
		// Pixverse has used `generate_audio` as an alias in some SDKs;
		// accept it here so customers don't get billed at the wrong tier
		// just because they typed the field name differently.
		hasAudio = boolFromMap(req.Metadata, "generate_audio")
	}

	return service.VideoBillingInput{
		OutputSeconds: duration,
		Width:         width,
		Height:        height,
		Resolution:    resolution,
		GroupRatio:    groupRatio,
		HasAudioInput: hasAudio,
	}
}

// resolveDimensions consults the profile's alias map, falling back to the
// profile's W/H defaults. Mirrors doubao.resolveVideoDimensions but
// scoped to this package to avoid cross-vendor coupling for one helper.
// AdjustBillingOnComplete re-runs CalculateVideoBilling without the
// pre-charge multiplier when a Pixverse task settles, so the customer
// is billed exactly the published $/sec × duration rate. Returns 0 to
// keep the conservative pre-charge for legacy / unconfigured models.
//
// Mirrors doubao.AdjustBillingOnComplete but specialised to the
// per_second path. Pixverse's result payload does not surface a
// `duration` field, so we trust the request-time duration captured in
// VideoParams (Pixverse C1 honours the requested duration deterministically).
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, _ *relaycommon.TaskInfo) int {
	if task == nil || task.PrivateData.BillingContext == nil {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if bc.BillingMode != videobilling.ModePerSecond {
		// Pre-pixverse-profile deployments left BillingMode empty and
		// relied on per-call ModelPrice; keep their behaviour intact.
		return 0
	}
	modelName := bc.BillingProfile
	if modelName == "" {
		modelName = bc.OriginModelName
	}
	profile, ok := videobilling.GetProfile(modelName)
	if !ok {
		return 0
	}
	groupRatio := bc.GroupRatio
	if groupRatio <= 0 {
		groupRatio = 1
	}

	input := videoBillingInputFromContext(bc.VideoParams, groupRatio)
	return service.CalculateVideoBilling(profile, input, false).Quota
}

func videoBillingInputFromContext(params map[string]any, groupRatio float64) service.VideoBillingInput {
	return service.VideoBillingInput{
		InputSeconds:  intFromMap(params, "input_seconds"),
		OutputSeconds: intFromMap(params, "output_seconds"),
		Width:         intFromMap(params, "width"),
		Height:        intFromMap(params, "height"),
		Resolution:    stringFromMap(params, "resolution"),
		GroupRatio:    groupRatio,
		HasAudioInput: boolFromMap(params, "has_audio_input"),
	}
}

func resolveDimensions(profile videobilling.VideoBillingProfile, alias string) (int, int) {
	if alias != "" && profile.ResolutionAliases != nil {
		if dim, ok := profile.ResolutionAliases[alias]; ok && dim.Width > 0 && dim.Height > 0 {
			return dim.Width, dim.Height
		}
	}
	return profile.FallbackWidth, profile.FallbackHeight
}

func normalizeResolutionAlias(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func firstPositiveInt(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func firstString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func atoi(raw string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(raw))
	return v
}

func intFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	switch v := values[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		return atoi(v)
	default:
		return 0
	}
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	v, _ := values[key].(string)
	return v
}

func boolFromMap(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	switch v := values[key].(type) {
	case bool:
		return v
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(v))
		return parsed
	default:
		return false
	}
}
