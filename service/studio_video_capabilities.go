package service

import (
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type StudioVideoModel struct {
	SchemaVersion string                       `json:"schemaVersion,omitempty"`
	ID            string                       `json:"id"`
	Label         string                       `json:"label"`
	Provider      string                       `json:"provider"`
	Versions      []StudioVideoModelVersion    `json:"versions"`
	Selection     *StudioVideoModelSelection   `json:"modelSelection,omitempty"`
	Modes         []StudioVideoMode            `json:"modes,omitempty"`
	Parameters    []StudioVideoParameter       `json:"parameters,omitempty"`
	TaskTypes     []string                     `json:"taskTypes"`
	Inputs        StudioVideoInputCapabilities `json:"inputs"`
	Settings      StudioVideoSettings          `json:"settings"`
	Batch         *StudioVideoBatchCapability  `json:"batch,omitempty"`
	PricingBasis  []string                     `json:"pricingBasis,omitempty"`
	Retention     *StudioVideoRetention        `json:"retention,omitempty"`
}

type StudioVideoModelVersion struct {
	ID        string                        `json:"id"`
	Label     string                        `json:"label"`
	Model     string                        `json:"model,omitempty"`
	Modes     []StudioVideoMode             `json:"modes,omitempty"`
	TaskTypes []string                      `json:"taskTypes,omitempty"`
	Inputs    *StudioVideoInputCapabilities `json:"inputs,omitempty"`
	Settings  *StudioVideoSettings          `json:"settings,omitempty"`
}

type StudioVideoModelSelection struct {
	DefaultModel            string            `json:"defaultModel,omitempty"`
	SupportedModelIDs       []string          `json:"supportedModelIds,omitempty"`
	RegionAliases           map[string]string `json:"regionAliases,omitempty"`
	PinnedSpecModelIDs      bool              `json:"pinnedSpecModelIds,omitempty"`
	QualityLockedByModelID  bool              `json:"qualityLockedByModelId,omitempty"`
	DurationLockedByModelID bool              `json:"durationLockedByModelId,omitempty"`
}

type StudioVideoMode struct {
	ID             string                 `json:"id"`
	Label          string                 `json:"label"`
	Supported      bool                   `json:"supported"`
	RequestMapping string                 `json:"requestMapping,omitempty"`
	RequiredInputs []StudioVideoModeInput `json:"requiredInputs,omitempty"`
	OptionalInputs []StudioVideoModeInput `json:"optionalInputs,omitempty"`
}

type StudioVideoModeInput struct {
	Role                    string   `json:"role"`
	MediaType               string   `json:"mediaType"`
	Min                     int      `json:"min,omitempty"`
	Max                     int      `json:"max,omitempty"`
	MaxTotalDurationSeconds int      `json:"maxTotalDurationSeconds,omitempty"`
	RequiresAnyOf           []string `json:"requiresAnyOf,omitempty"`
}

type StudioVideoInputCapability struct {
	Supported               bool     `json:"supported"`
	MediaTypes              []string `json:"mediaTypes,omitempty"`
	Formats                 []string `json:"formats,omitempty"`
	Min                     int      `json:"min,omitempty"`
	Max                     int      `json:"max,omitempty"`
	MaxFileSizeMB           int      `json:"maxFileSizeMB,omitempty"`
	MinDurationSeconds      int      `json:"minDurationSeconds,omitempty"`
	MaxDurationSeconds      int      `json:"maxDurationSeconds,omitempty"`
	MaxTotalDurationSeconds int      `json:"maxTotalDurationSeconds,omitempty"`
	RequiresMediaInput      bool     `json:"requiresMediaInput,omitempty"`
	MaxLength               int      `json:"maxLength,omitempty"`
}

type StudioVideoInputCapabilities struct {
	Prompt          StudioVideoInputCapability `json:"prompt,omitempty"`
	FirstFrame      StudioVideoInputCapability `json:"firstFrame,omitempty"`
	LastFrame       StudioVideoInputCapability `json:"lastFrame,omitempty"`
	ReferenceImages StudioVideoInputCapability `json:"referenceImages,omitempty"`
	ReferenceVideos StudioVideoInputCapability `json:"referenceVideos,omitempty"`
	ReferenceAudios StudioVideoInputCapability `json:"referenceAudios,omitempty"`
}

type StudioVideoSettings struct {
	Durations         []int    `json:"durations"`
	DurationMin       int      `json:"durationMin,omitempty"`
	DurationMax       int      `json:"durationMax,omitempty"`
	DefaultDuration   int      `json:"defaultDuration,omitempty"`
	Ratios            []string `json:"ratios"`
	DefaultRatio      string   `json:"defaultRatio,omitempty"`
	Resolutions       []string `json:"resolutions"`
	DefaultResolution string   `json:"defaultResolution,omitempty"`
	GenerateAudio     bool     `json:"generateAudio,omitempty"`
	WebSearch         bool     `json:"webSearch,omitempty"`
	Seed              bool     `json:"seed,omitempty"`
	RandomSeed        bool     `json:"randomSeed,omitempty"`
	ReturnLastFrame   bool     `json:"returnLastFrame,omitempty"`
	Watermark         bool     `json:"watermark,omitempty"`
	CameraFixed       bool     `json:"cameraFixed,omitempty"`
}

type StudioVideoParameter struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Values      []string `json:"values,omitempty"`
	Min         *int     `json:"min,omitempty"`
	Max         *int     `json:"max,omitempty"`
	Default     string   `json:"default,omitempty"`
	Modes       []string `json:"modes,omitempty"`
	IgnoredWhen []string `json:"ignoredWhen,omitempty"`
	Description string   `json:"description,omitempty"`
}

type StudioVideoBatchCapability struct {
	Supported bool `json:"supported"`
	MinCount  int  `json:"minCount"`
	MaxCount  int  `json:"maxCount"`
	Native    bool `json:"native"`
}

type StudioVideoRetention struct {
	GeneratedOutput string `json:"generatedOutput,omitempty"`
	TemporaryInput  bool   `json:"temporaryInput,omitempty"`
}

func ListStudioVideoModels(userID int, tokenModelLimit map[string]bool) ([]StudioVideoModel, error) {
	user, err := model.GetUserCache(userID)
	if err != nil {
		return nil, err
	}

	groups := GetUserUsableGroups(user.Group)
	seenModelNames := map[string]bool{}
	modelIndex := map[string]int{}
	models := make([]StudioVideoModel, 0)

	for group := range groups {
		for _, enabledModel := range model.GetGroupEnabledModels(group) {
			if seenModelNames[enabledModel] || !tokenAllowsModel(tokenModelLimit, enabledModel) {
				continue
			}
			seenModelNames[enabledModel] = true
			capability, ok := DescribeStudioVideoModel(enabledModel)
			if !ok {
				continue
			}
			if index, exists := modelIndex[capability.ID]; exists {
				models[index].Versions = mergeStudioVideoVersions(models[index].Versions, capability.Versions)
				continue
			}
			modelIndex[capability.ID] = len(models)
			models = append(models, capability)
		}
	}

	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Label == models[j].Label {
			return models[i].ID < models[j].ID
		}
		return models[i].Label < models[j].Label
	})
	for index := range models {
		sort.SliceStable(models[index].Versions, func(i, j int) bool {
			return studioVideoVersionRank(models[index].Versions[i].ID) < studioVideoVersionRank(models[index].Versions[j].ID)
		})
	}
	return models, nil
}

func DescribeStudioVideoModel(modelName string) (StudioVideoModel, bool) {
	lower := strings.ToLower(modelName)
	switch {
	case strings.Contains(lower, "seedance") || strings.Contains(lower, "doubao"):
		return seedanceStudioVideoModel(modelName), true
	case strings.Contains(lower, "pixverse"):
		return pixverseStudioVideoModel(modelName), true
	case strings.Contains(lower, "kling"):
		return genericStudioVideoModel(modelName, "kling"), true
	case strings.Contains(lower, "sora"):
		return genericStudioVideoModel(modelName, "sora"), true
	case strings.Contains(lower, "vidu"):
		return genericStudioVideoModel(modelName, "vidu"), true
	case strings.Contains(lower, "hailuo"):
		return genericStudioVideoModel(modelName, "hailuo"), true
	case strings.Contains(lower, "wan"):
		return genericStudioVideoModel(modelName, "wan"), true
	default:
		return StudioVideoModel{}, false
	}
}

func tokenAllowsModel(tokenModelLimit map[string]bool, modelName string) bool {
	return len(tokenModelLimit) == 0 || tokenModelLimit[modelName]
}

func seedanceStudioVideoModel(modelName string) StudioVideoModel {
	isFast := strings.Contains(strings.ToLower(modelName), "fast")
	versionID := "default"
	versionLabel := "Standard"
	canonicalID := modelName
	if isFast {
		versionID = "fast"
		versionLabel = "Fast"
		canonicalID = removeCaseInsensitive(modelName, "-fast")
	}
	inputs := seedanceStudioVideoInputs()
	settings := seedanceStudioVideoSettings(isFast)
	modes := seedanceStudioVideoModes(isFast)
	taskTypes := seedanceStudioVideoTaskTypes(isFast)
	return StudioVideoModel{
		SchemaVersion: "2026-05-21",
		ID:            canonicalID,
		Label:         "Seedance 2.0",
		Provider:      "doubao",
		Selection: &StudioVideoModelSelection{
			DefaultModel:      "doubao-seedance-2-0-260128",
			SupportedModelIDs: []string{"doubao-seedance-2-0-260128", "doubao-seedance-2-0-fast-260128", "seedance-2.0-global", "seedance-2.0-fast-global"},
			RegionAliases: map[string]string{
				"doubao-seedance-2-0-260128":      "seedance-2.0-global",
				"doubao-seedance-2-0-fast-260128": "seedance-2.0-fast-global",
			},
		},
		Modes:      modes,
		Parameters: seedanceStudioVideoParameters(),
		Versions: []StudioVideoModelVersion{{
			ID:        versionID,
			Label:     versionLabel,
			Model:     modelName,
			Modes:     modes,
			TaskTypes: taskTypes,
			Inputs:    &inputs,
			Settings:  &settings,
		}},
		TaskTypes:    taskTypes,
		Inputs:       inputs,
		Settings:     settings,
		Batch:        &StudioVideoBatchCapability{Supported: true, MinCount: 1, MaxCount: 4, Native: false},
		PricingBasis: []string{"duration_seconds", "resolution", "has_video_input", "has_audio_input", "batch_count"},
		Retention:    &StudioVideoRetention{GeneratedOutput: "upstream_proxy", TemporaryInput: true},
	}
}

func pixverseStudioVideoModel(modelName string) StudioVideoModel {
	quality := pixversePinnedQuality(modelName)
	duration := pixversePinnedDuration(modelName)
	inputs := StudioVideoInputCapabilities{
		Prompt:          StudioVideoInputCapability{Supported: true, MaxLength: 2048},
		FirstFrame:      StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
		LastFrame:       StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
		ReferenceImages: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 9},
		ReferenceVideos: StudioVideoInputCapability{Supported: false},
		ReferenceAudios: StudioVideoInputCapability{Supported: false},
	}
	settings := StudioVideoSettings{
		Durations:         pixverseDurations(duration),
		DurationMin:       1,
		DurationMax:       15,
		DefaultDuration:   5,
		Ratios:            []string{"16:9", "9:16", "1:1", "4:3", "3:4", "2:3", "3:2", "21:9"},
		DefaultRatio:      "adaptive",
		Resolutions:       pixverseResolutions(quality),
		DefaultResolution: "540p",
		GenerateAudio:     true,
		Seed:              true,
	}
	if duration > 0 {
		settings.DurationMin = duration
		settings.DurationMax = duration
		settings.DefaultDuration = duration
	}
	if quality != "" {
		settings.DefaultResolution = quality
	}
	modes := []StudioVideoMode{
		{ID: "text_to_video", Label: "Text to video", Supported: true, RequestMapping: "prompt"},
		{ID: "image_first_frame", Label: "Image to video", Supported: true, RequestMapping: "top_level_images", RequiredInputs: []StudioVideoModeInput{{Role: "first_frame", MediaType: "image", Min: 1, Max: 1}}},
		{ID: "image_first_last_frame", Label: "First/last frame", Supported: true, RequestMapping: "top_level_images", RequiredInputs: []StudioVideoModeInput{{Role: "first_frame", MediaType: "image", Min: 1, Max: 1}, {Role: "last_frame", MediaType: "image", Min: 1, Max: 1}}},
		{ID: "multimodal_reference", Label: "Multi-reference fusion", Supported: true, RequestMapping: "top_level_images_with_metadata_references", RequiredInputs: []StudioVideoModeInput{{Role: "reference_image", MediaType: "image", Min: 3, Max: 9}}},
	}
	return StudioVideoModel{
		SchemaVersion: "2026-05-21",
		ID:            modelName,
		Label:         modelName,
		Provider:      "pixverse",
		Selection: &StudioVideoModelSelection{
			DefaultModel:            "pixverse-v4.5",
			SupportedModelIDs:       []string{"pixverse-v3.5", "pixverse-v4", "pixverse-v4.5", "pixverse-v5", "pixverse-v5.5", "pixverse-v5.6", "pixverse-c1", "pixverse-v6"},
			PinnedSpecModelIDs:      true,
			QualityLockedByModelID:  quality != "",
			DurationLockedByModelID: duration > 0,
		},
		Modes:      modes,
		Parameters: pixverseStudioVideoParameters(),
		Versions: []StudioVideoModelVersion{{
			ID:        "default",
			Label:     "Standard",
			Model:     modelName,
			Modes:     modes,
			TaskTypes: []string{"text_to_video", "image_to_video"},
			Inputs:    &inputs,
			Settings:  &settings,
		}},
		TaskTypes:    []string{"text_to_video", "image_to_video"},
		Inputs:       inputs,
		Settings:     settings,
		Batch:        &StudioVideoBatchCapability{Supported: true, MinCount: 1, MaxCount: 4, Native: false},
		PricingBasis: []string{"duration_seconds", "resolution", "has_image_input", "batch_count", "pinned_model_sku"},
		Retention:    &StudioVideoRetention{GeneratedOutput: "upstream_proxy", TemporaryInput: true},
	}
}

func genericStudioVideoModel(modelName string, provider string) StudioVideoModel {
	inputs := StudioVideoInputCapabilities{
		Prompt:          StudioVideoInputCapability{Supported: true, MaxLength: 4000},
		FirstFrame:      StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
		LastFrame:       StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
		ReferenceImages: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 4},
		ReferenceVideos: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"video"}, Max: 1},
		ReferenceAudios: StudioVideoInputCapability{Supported: false},
	}
	settings := StudioVideoSettings{
		Durations:         intRange(5, 10),
		DurationMin:       5,
		DurationMax:       10,
		DefaultDuration:   5,
		Ratios:            []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"},
		DefaultRatio:      "16:9",
		Resolutions:       []string{"480p", "720p", "1080p"},
		DefaultResolution: "720p",
		GenerateAudio:     true,
		Seed:              true,
		RandomSeed:        true,
	}
	modes := []StudioVideoMode{
		{ID: "text_to_video", Label: "Text to video", Supported: true, RequestMapping: "prompt"},
		{ID: "image_first_frame", Label: "Image to video", Supported: true, RequestMapping: "top_level_images", RequiredInputs: []StudioVideoModeInput{{Role: "first_frame", MediaType: "image", Min: 1, Max: 1}}},
		{ID: "image_first_last_frame", Label: "First/last frame", Supported: true, RequestMapping: "metadata_content_roles", RequiredInputs: []StudioVideoModeInput{{Role: "first_frame", MediaType: "image", Min: 1, Max: 1}, {Role: "last_frame", MediaType: "image", Min: 1, Max: 1}}},
		{ID: "multimodal_reference", Label: "Multi-reference", Supported: true, RequestMapping: "metadata_content_roles", OptionalInputs: []StudioVideoModeInput{{Role: "reference_image", MediaType: "image", Max: 4}, {Role: "reference_video", MediaType: "video", Max: 1}}},
	}
	return StudioVideoModel{
		SchemaVersion: "2026-05-21",
		ID:            modelName,
		Label:         modelName,
		Provider:      provider,
		Modes:         modes,
		Versions: []StudioVideoModelVersion{{
			ID:        "default",
			Label:     "Standard",
			Model:     modelName,
			Modes:     modes,
			TaskTypes: []string{"text_to_video", "image_to_video", "video_to_video"},
			Inputs:    &inputs,
			Settings:  &settings,
		}},
		TaskTypes:    []string{"text_to_video", "image_to_video", "video_to_video"},
		Inputs:       inputs,
		Settings:     settings,
		Batch:        &StudioVideoBatchCapability{Supported: true, MinCount: 1, MaxCount: 4, Native: false},
		PricingBasis: []string{"duration_seconds", "resolution", "has_video_input", "batch_count"},
		Retention:    &StudioVideoRetention{GeneratedOutput: "upstream_proxy", TemporaryInput: true},
	}
}

func seedanceStudioVideoInputs() StudioVideoInputCapabilities {
	return StudioVideoInputCapabilities{
		Prompt: StudioVideoInputCapability{Supported: true, MaxLength: 4000},
		FirstFrame: StudioVideoInputCapability{
			Supported:     true,
			MediaTypes:    []string{"image"},
			Formats:       []string{"jpeg", "png", "webp", "bmp", "tiff", "gif"},
			Min:           1,
			Max:           1,
			MaxFileSizeMB: 30,
		},
		LastFrame: StudioVideoInputCapability{
			Supported:     true,
			MediaTypes:    []string{"image"},
			Formats:       []string{"jpeg", "png", "webp", "bmp", "tiff", "gif"},
			Max:           1,
			MaxFileSizeMB: 30,
		},
		ReferenceImages: StudioVideoInputCapability{
			Supported:     true,
			MediaTypes:    []string{"image"},
			Formats:       []string{"jpeg", "png", "webp", "bmp", "tiff", "gif"},
			Max:           9,
			MaxFileSizeMB: 30,
		},
		ReferenceVideos: StudioVideoInputCapability{
			Supported:               true,
			MediaTypes:              []string{"video"},
			Formats:                 []string{"mp4", "mov"},
			Max:                     3,
			MaxFileSizeMB:           50,
			MinDurationSeconds:      2,
			MaxDurationSeconds:      15,
			MaxTotalDurationSeconds: 15,
		},
		ReferenceAudios: StudioVideoInputCapability{
			Supported:               true,
			MediaTypes:              []string{"audio"},
			Formats:                 []string{"wav", "mp3"},
			Max:                     3,
			MaxFileSizeMB:           15,
			MinDurationSeconds:      2,
			MaxDurationSeconds:      15,
			MaxTotalDurationSeconds: 15,
			RequiresMediaInput:      true,
		},
	}
}

func seedanceStudioVideoSettings(isFast bool) StudioVideoSettings {
	maxDuration := 15
	resolutions := []string{"480p", "720p", "1080p"}
	if isFast {
		maxDuration = 12
		resolutions = []string{"480p", "720p"}
	}
	return StudioVideoSettings{
		Durations:         intRange(4, maxDuration),
		DurationMin:       4,
		DurationMax:       maxDuration,
		DefaultDuration:   5,
		Ratios:            []string{"adaptive", "16:9", "9:16", "1:1", "4:3", "3:4", "21:9"},
		DefaultRatio:      "16:9",
		Resolutions:       resolutions,
		DefaultResolution: "720p",
		GenerateAudio:     true,
		WebSearch:         !isFast,
		Seed:              true,
		RandomSeed:        true,
		ReturnLastFrame:   true,
		Watermark:         true,
		CameraFixed:       false,
	}
}

func seedanceStudioVideoModes(isFast bool) []StudioVideoMode {
	modes := []StudioVideoMode{
		{ID: "text_to_video", Label: "Text to video", Supported: true, RequestMapping: "prompt"},
		{ID: "image_first_frame", Label: "Image to video · first frame", Supported: true, RequestMapping: "top_level_images", RequiredInputs: []StudioVideoModeInput{{Role: "first_frame", MediaType: "image", Min: 1, Max: 1}}},
		{ID: "image_first_last_frame", Label: "Image to video · first/last frame", Supported: true, RequestMapping: "metadata_content_roles", RequiredInputs: []StudioVideoModeInput{{Role: "first_frame", MediaType: "image", Min: 1, Max: 1}, {Role: "last_frame", MediaType: "image", Min: 1, Max: 1}}},
		{ID: "multimodal_reference", Label: "Multi-modal reference", Supported: true, RequestMapping: "metadata_content_roles", OptionalInputs: []StudioVideoModeInput{{Role: "reference_image", MediaType: "image", Max: 9}, {Role: "reference_video", MediaType: "video", Max: 3, MaxTotalDurationSeconds: 15}, {Role: "reference_audio", MediaType: "audio", Max: 3, MaxTotalDurationSeconds: 15, RequiresAnyOf: []string{"reference_image", "reference_video"}}}},
		{ID: "edit_video", Label: "Edit video", Supported: !isFast, RequestMapping: "metadata_content_roles", RequiredInputs: []StudioVideoModeInput{{Role: "reference_video", MediaType: "video", Min: 1, Max: 1}}, OptionalInputs: []StudioVideoModeInput{{Role: "reference_image", MediaType: "image", Max: 1}}},
		{ID: "extend_video", Label: "Extend video", Supported: !isFast, RequestMapping: "metadata_content_roles", RequiredInputs: []StudioVideoModeInput{{Role: "reference_video", MediaType: "video", Min: 1, Max: 3, MaxTotalDurationSeconds: 15}}},
		{ID: "web_search", Label: "Web search augmented", Supported: !isFast, RequestMapping: "metadata_tools"},
	}
	return modes
}

func seedanceStudioVideoTaskTypes(isFast bool) []string {
	taskTypes := []string{"text_to_video", "image_to_video", "video_to_video"}
	if !isFast {
		taskTypes = append(taskTypes, "edit_video", "extend_video", "web_search")
	}
	return taskTypes
}

func seedanceStudioVideoParameters() []StudioVideoParameter {
	promptMax := 4000
	durationMin := 4
	durationMax := 15
	seedMin := 0
	return []StudioVideoParameter{
		{Name: "model", Type: "string", Required: true, Description: "Seedance model ID."},
		{Name: "prompt", Type: "string", Max: &promptMax, Description: "Text prompt; overly long prompts may be partially ignored."},
		{Name: "size", Type: "string", Values: []string{"480p", "720p", "1080p"}, Default: "720p", Description: "Output resolution; 1080p is unavailable on fast models."},
		{Name: "duration", Type: "integer", Min: &durationMin, Max: &durationMax, Description: "Duration in seconds; -1 lets the model decide when supported."},
		{Name: "images", Type: "string[]", Modes: []string{"image_first_frame"}, Description: "Sugar for one first_frame image."},
		{Name: "metadata.content", Type: "array", Modes: []string{"image_first_last_frame", "multimodal_reference", "edit_video", "extend_video"}, Description: "Content entries with type, URL payload, and role."},
		{Name: "metadata.ratio", Type: "string", Values: []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}, Default: "adaptive", Description: "Output aspect ratio."},
		{Name: "metadata.generate_audio", Type: "boolean", Default: "true", Description: "Generate a synced audio track."},
		{Name: "metadata.tools", Type: "array", Modes: []string{"web_search"}, Values: []string{"web_search"}, Description: "Tool list for web-search augmented generation."},
		{Name: "metadata.seed", Type: "integer", Min: &seedMin, Description: "Reproducibility seed."},
		{Name: "metadata.watermark", Type: "boolean", Default: "false", Description: "Whether to overlay a watermark."},
		{Name: "metadata.return_last_frame", Type: "boolean", Description: "Return a still frame from the end of the video."},
		{Name: "metadata.audit_image", Type: "boolean", Description: "Audit real-person reference images before upstream submission."},
	}
}

func pixverseStudioVideoParameters() []StudioVideoParameter {
	promptMax := 2048
	durationMin := 1
	durationMax := 15
	seedMin := 0
	seedMax := 2147483647
	return []StudioVideoParameter{
		{Name: "model", Type: "string", Required: true, Description: "Pixverse model ID; suffixes can pin quality and duration."},
		{Name: "prompt", Type: "string", Required: true, Max: &promptMax, Description: "Text prompt; use @ref_name in fusion mode."},
		{Name: "images", Type: "string[]", Modes: []string{"image_first_frame", "image_first_last_frame", "multimodal_reference"}, Description: "0=text, 1=image-to-video, 2=transition, 3+=fusion. URLs, data URIs, or Pixverse img_id strings."},
		{Name: "size", Type: "string", IgnoredWhen: []string{"model id pins quality"}, Description: "WxH hint mapped to quality and aspect_ratio."},
		{Name: "duration", Type: "integer", Min: &durationMin, Max: &durationMax, Default: "5", IgnoredWhen: []string{"model id pins duration"}, Description: "Output duration in seconds."},
		{Name: "metadata.quality", Type: "string", Values: []string{"360p", "540p", "720p", "1080p"}, Default: "540p", IgnoredWhen: []string{"model id pins quality"}, Description: "Output quality."},
		{Name: "metadata.aspect_ratio", Type: "string", Values: []string{"16:9", "9:16", "1:1", "4:3", "3:4", "2:3", "3:2", "21:9"}, Default: "16:9", Description: "Output aspect ratio."},
		{Name: "metadata.motion_mode", Type: "string", Values: []string{"normal", "fast"}, Description: "Fast motion mode is limited to 5s and <=720p."},
		{Name: "metadata.negative_prompt", Type: "string", Description: "Things to avoid in the output."},
		{Name: "metadata.seed", Type: "integer", Min: &seedMin, Max: &seedMax, Description: "Reproducibility seed."},
		{Name: "metadata.water_mark", Type: "boolean", Description: "Include Pixverse watermark."},
		{Name: "metadata.generate_audio_switch", Type: "boolean", Description: "Add generated audio when supported by the selected model."},
		{Name: "metadata.style", Type: "string", Description: "Pixverse style name."},
		{Name: "metadata.img_id", Type: "integer", Modes: []string{"image_first_frame"}, Description: "Pre-uploaded image ID for image-to-video."},
		{Name: "metadata.first_frame_img", Type: "integer", Modes: []string{"image_first_last_frame"}, Description: "Pre-uploaded first-frame image ID."},
		{Name: "metadata.last_frame_img", Type: "integer", Modes: []string{"image_first_last_frame"}, Description: "Pre-uploaded last-frame image ID."},
		{Name: "metadata.image_references", Type: "object[]", Modes: []string{"multimodal_reference"}, Description: "Fusion references shaped as {type,img_id,ref_name}."},
	}
}

func pixverseDurations(pinned int) []int {
	if pinned > 0 {
		return []int{pinned}
	}
	return []int{1, 5, 8, 15}
}

func pixverseResolutions(pinned string) []string {
	if pinned != "" {
		return []string{pinned}
	}
	return []string{"360p", "540p", "720p", "1080p"}
}

func pixversePinnedQuality(modelName string) string {
	lower := strings.ToLower(modelName)
	for _, quality := range []string{"360p", "540p", "720p", "1080p"} {
		if strings.Contains(lower, "-"+quality) {
			return quality
		}
	}
	return ""
}

func pixversePinnedDuration(modelName string) int {
	lower := strings.ToLower(modelName)
	for _, duration := range []int{5, 8, 10, 15} {
		if strings.Contains(lower, "-"+strconv.Itoa(duration)+"s") {
			return duration
		}
	}
	return 0
}

func intRange(min int, max int) []int {
	if max < min {
		return nil
	}
	values := make([]int, 0, max-min+1)
	for value := min; value <= max; value++ {
		values = append(values, value)
	}
	return values
}

func mergeStudioVideoVersions(existing []StudioVideoModelVersion, additions []StudioVideoModelVersion) []StudioVideoModelVersion {
	seen := map[string]bool{}
	for _, version := range existing {
		seen[version.ID] = true
	}
	for _, version := range additions {
		if !seen[version.ID] {
			existing = append(existing, version)
			seen[version.ID] = true
		}
	}
	return existing
}

func studioVideoVersionRank(versionID string) int {
	switch versionID {
	case "default", "standard":
		return 0
	case "fast":
		return 1
	default:
		return 10
	}
}

func removeCaseInsensitive(value string, target string) string {
	index := strings.Index(strings.ToLower(value), strings.ToLower(target))
	if index < 0 {
		return value
	}
	return value[:index] + value[index+len(target):]
}
