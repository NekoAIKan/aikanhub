package doubao

import (
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
)

func ExtractRequestBillingInput(req relaycommon.TaskSubmitReq, profile videobilling.VideoBillingProfile, groupRatio float64) service.VideoBillingInput {
	outputSeconds := firstPositiveInt(intFromMap(req.Metadata, "duration"), req.Duration, atoi(req.Seconds), intFromMap(req.Metadata, "seconds"))
	width, height := resolveRequestVideoDimensions(profile, req)
	fps := firstPositiveInt(intFromMap(req.Metadata, "fps"), intFromMap(req.Metadata, "framespersecond"), profile.FallbackFPS)
	draft := boolFromMap(req.Metadata, "draft")

	inputSeconds := firstPositiveInt(
		intFromMap(req.Metadata, "input_seconds"),
		intFromMap(req.Metadata, "input_duration"),
		intFromMap(req.Metadata, "input_video_duration"),
	)
	hasReferenceMedia := strings.TrimSpace(req.InputReference) != "" ||
		metadataHasVideoInput(req.Metadata) ||
		contentHasVideoInput(req.Content)
	if inputSeconds <= 0 && hasReferenceMedia {
		inputSeconds = firstPositiveInt(outputSeconds, profile.FallbackDurationSeconds)
	}
	if inputSeconds > 0 {
		hasReferenceMedia = true
	}

	return service.VideoBillingInput{
		InputSeconds:      inputSeconds,
		OutputSeconds:     outputSeconds,
		Width:             width,
		Height:            height,
		FPS:               fps,
		GroupRatio:        groupRatio,
		Draft:             draft,
		HasReferenceMedia: hasReferenceMedia,
	}
}

func ExtractResponseBillingInput(body []byte, profile videobilling.VideoBillingProfile, groupRatio float64) (service.VideoBillingInput, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(body, &resTask); err != nil {
		return service.VideoBillingInput{}, err
	}
	width, height := resolveVideoDimensionsWithRatio(profile, resTask.Resolution, resTask.Ratio)
	fps := firstPositiveInt(resTask.FramesPerSecond, profile.FallbackFPS)
	return service.VideoBillingInput{
		OutputSeconds:       firstPositiveInt(resTask.Duration, profile.FallbackDurationSeconds),
		Width:               width,
		Height:              height,
		FPS:                 fps,
		GroupRatio:          groupRatio,
		UpstreamTotalTokens: resTask.Usage.TotalTokens,
	}, nil
}

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || task.PrivateData.BillingContext == nil {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if bc.BillingMode != videobilling.ModeFormula && bc.BillingMode != "video_formula" {
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
	if bc.RetailUnitPrice > 0 {
		profile.UnitPrice = bc.RetailUnitPrice
	}
	groupRatio := bc.GroupRatio
	if groupRatio <= 0 {
		groupRatio = 1
	}

	input := videoBillingInputFromContext(bc.VideoParams, groupRatio)
	if len(task.Data) > 0 {
		if responseInput, err := ExtractResponseBillingInput(task.Data, profile, groupRatio); err == nil {
			input = mergeVideoBillingInput(input, responseInput)
		}
	}
	if taskResult != nil && taskResult.TotalTokens > 0 {
		input.UpstreamTotalTokens = taskResult.TotalTokens
	}

	return service.CalculateVideoBilling(profile, input, false).Quota
}

func videoBillingInputFromContext(params map[string]any, groupRatio float64) service.VideoBillingInput {
	return service.VideoBillingInput{
		InputSeconds:        intFromMap(params, "input_seconds"),
		OutputSeconds:       intFromMap(params, "output_seconds"),
		Width:               intFromMap(params, "width"),
		Height:              intFromMap(params, "height"),
		FPS:                 intFromMap(params, "fps"),
		GroupRatio:          groupRatio,
		Draft:               boolFromMap(params, "draft"),
		UpstreamTotalTokens: intFromMap(params, "upstream_total_tokens"),
		HasReferenceMedia:   boolFromMap(params, "has_reference_media"),
	}
}

func mergeVideoBillingInput(base, override service.VideoBillingInput) service.VideoBillingInput {
	if override.OutputSeconds > 0 {
		base.OutputSeconds = override.OutputSeconds
	}
	if override.Width > 0 {
		base.Width = override.Width
	}
	if override.Height > 0 {
		base.Height = override.Height
	}
	if override.FPS > 0 {
		base.FPS = override.FPS
	}
	if override.UpstreamTotalTokens > 0 {
		base.UpstreamTotalTokens = override.UpstreamTotalTokens
	}
	if override.HasReferenceMedia {
		base.HasReferenceMedia = true
	}
	if base.GroupRatio <= 0 {
		base.GroupRatio = override.GroupRatio
	}
	return base
}

func resolveVideoDimensions(profile videobilling.VideoBillingProfile, raw string) (int, int) {
	return resolveVideoDimensionsWithRatio(profile, raw, "")
}

func resolveRequestVideoDimensions(profile videobilling.VideoBillingProfile, req relaycommon.TaskSubmitReq) (int, int) {
	resolution := firstString(stringFromMap(req.Metadata, "resolution"), req.Resolution)
	ratio := firstString(stringFromMap(req.Metadata, "ratio"), stringFromMap(req.Metadata, "aspect_ratio"))
	if resolution != "" || ratio != "" {
		return resolveVideoDimensionsWithRatio(profile, firstString(resolution, req.Size, stringFromMap(req.Metadata, "size")), ratio)
	}
	for _, size := range []string{req.Size, stringFromMap(req.Metadata, "size")} {
		if width, height, ok := parseResolutionPair(size); ok {
			return width, height
		}
	}
	return resolveVideoDimensionsWithRatio(profile, firstString(req.Size, stringFromMap(req.Metadata, "size")), ratio)
}

func resolveVideoDimensionsWithRatio(profile videobilling.VideoBillingProfile, raw string, ratio string) (int, int) {
	width, height := resolveVideoDimensionsBase(profile, raw)
	if ratio == "" || ratio == "adaptive" {
		return width, height
	}
	ratioWidth, ratioHeight, ok := parseAspectRatio(ratio)
	if !ok {
		return width, height
	}
	area := float64(width * height)
	if area <= 0 {
		return width, height
	}
	adjustedWidth := int(math.Round(math.Sqrt(area * float64(ratioWidth) / float64(ratioHeight))))
	adjustedHeight := int(math.Round(math.Sqrt(area * float64(ratioHeight) / float64(ratioWidth))))
	if adjustedWidth <= 0 || adjustedHeight <= 0 {
		return width, height
	}
	return adjustedWidth, adjustedHeight
}

func resolveVideoDimensionsBase(profile videobilling.VideoBillingProfile, raw string) (int, int) {
	if raw != "" {
		key := strings.ToLower(strings.TrimSpace(raw))
		for alias, resolution := range profile.ResolutionAliases {
			if strings.ToLower(strings.TrimSpace(alias)) == key {
				return resolution.Width, resolution.Height
			}
		}
		if width, height, ok := parseResolutionPair(key); ok {
			return width, height
		}
	}
	return profile.FallbackWidth, profile.FallbackHeight
}

func parseAspectRatio(raw string) (int, int, bool) {
	parts := strings.SplitN(strings.TrimSpace(raw), ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, errWidth := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, errHeight := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errWidth != nil || errHeight != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func parseResolutionPair(raw string) (int, int, bool) {
	normalized := strings.NewReplacer("*", "x", "X", "x", " ", "").Replace(raw)
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, errWidth := strconv.Atoi(parts[0])
	height, errHeight := strconv.Atoi(parts[1])
	if errWidth != nil || errHeight != nil || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func metadataHasVideoInput(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	if _, ok := metadata["video_url"]; ok {
		return true
	}
	switch content := metadata["content"].(type) {
	case []any:
		for _, item := range content {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if itemMap["type"] == "video_url" {
				return true
			}
			if _, ok := itemMap["video_url"]; ok {
				return true
			}
		}
	case []map[string]any:
		return contentHasVideoInput(content)
	}
	return false
}

func contentHasVideoInput(content []map[string]any) bool {
	for _, itemMap := range content {
		if itemMap["type"] == "video_url" {
			return true
		}
		if _, ok := itemMap["video_url"]; ok {
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

func atoi(raw string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(raw))
	return value
}

func intFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		return atoi(value)
	default:
		return 0
	}
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func boolFromMap(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	switch value := values[key].(type) {
	case bool:
		return value
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
		return parsed
	default:
		return false
	}
}
