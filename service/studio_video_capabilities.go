package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type StudioVideoModel struct {
	ID           string                       `json:"id"`
	Label        string                       `json:"label"`
	Provider     string                       `json:"provider"`
	Versions     []StudioVideoModelVersion    `json:"versions"`
	TaskTypes    []string                     `json:"taskTypes"`
	Inputs       StudioVideoInputCapabilities `json:"inputs"`
	Settings     StudioVideoSettings          `json:"settings"`
	Batch        *StudioVideoBatchCapability  `json:"batch,omitempty"`
	PricingBasis []string                     `json:"pricingBasis,omitempty"`
	Retention    *StudioVideoRetention        `json:"retention,omitempty"`
}

type StudioVideoModelVersion struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Model string `json:"model,omitempty"`
}

type StudioVideoInputCapability struct {
	Supported               bool     `json:"supported"`
	MediaTypes              []string `json:"mediaTypes,omitempty"`
	Min                     int      `json:"min,omitempty"`
	Max                     int      `json:"max,omitempty"`
	MaxFileSizeMB           int      `json:"maxFileSizeMB,omitempty"`
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
	Durations       []int    `json:"durations"`
	Ratios          []string `json:"ratios"`
	Resolutions     []string `json:"resolutions"`
	GenerateAudio   bool     `json:"generateAudio,omitempty"`
	WebSearch       bool     `json:"webSearch,omitempty"`
	Seed            bool     `json:"seed,omitempty"`
	RandomSeed      bool     `json:"randomSeed,omitempty"`
	ReturnLastFrame bool     `json:"returnLastFrame,omitempty"`
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
		return genericStudioVideoModel(modelName, "pixverse"), true
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
	versionID := "default"
	versionLabel := "Standard"
	canonicalID := modelName
	if strings.Contains(strings.ToLower(modelName), "fast") {
		versionID = "fast"
		versionLabel = "Fast"
		canonicalID = removeCaseInsensitive(modelName, "-fast")
	}
	return StudioVideoModel{
		ID:       canonicalID,
		Label:    "Seedance 2.0",
		Provider: "doubao",
		Versions: []StudioVideoModelVersion{{
			ID:    versionID,
			Label: versionLabel,
			Model: modelName,
		}},
		TaskTypes: []string{"text_to_video", "image_to_video", "video_to_video"},
		Inputs: StudioVideoInputCapabilities{
			Prompt:          StudioVideoInputCapability{Supported: true, MaxLength: 4000},
			FirstFrame:      StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
			LastFrame:       StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
			ReferenceImages: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 9},
			ReferenceVideos: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"video"}, Max: 3, MaxTotalDurationSeconds: 15},
			ReferenceAudios: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"audio"}, Max: 3, RequiresMediaInput: true},
		},
		Settings: StudioVideoSettings{
			Durations:     []int{5, 8, 10},
			Ratios:        []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"},
			Resolutions:   []string{"480p", "720p", "1080p"},
			GenerateAudio: true,
			Seed:          true,
			RandomSeed:    true,
		},
		Batch:        &StudioVideoBatchCapability{Supported: true, MinCount: 1, MaxCount: 4, Native: false},
		PricingBasis: []string{"duration_seconds", "resolution", "has_video_input", "has_audio_input", "batch_count"},
		Retention:    &StudioVideoRetention{GeneratedOutput: "upstream_proxy", TemporaryInput: true},
	}
}

func genericStudioVideoModel(modelName string, provider string) StudioVideoModel {
	return StudioVideoModel{
		ID:       modelName,
		Label:    modelName,
		Provider: provider,
		Versions: []StudioVideoModelVersion{{
			ID:    "default",
			Label: "Standard",
			Model: modelName,
		}},
		TaskTypes: []string{"text_to_video", "image_to_video", "video_to_video"},
		Inputs: StudioVideoInputCapabilities{
			Prompt:          StudioVideoInputCapability{Supported: true, MaxLength: 4000},
			FirstFrame:      StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
			LastFrame:       StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 1},
			ReferenceImages: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"image"}, Max: 4},
			ReferenceVideos: StudioVideoInputCapability{Supported: true, MediaTypes: []string{"video"}, Max: 1},
			ReferenceAudios: StudioVideoInputCapability{Supported: false},
		},
		Settings: StudioVideoSettings{
			Durations:     []int{5, 8, 10},
			Ratios:        []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"},
			Resolutions:   []string{"480p", "720p", "1080p"},
			GenerateAudio: true,
			Seed:          true,
			RandomSeed:    true,
		},
		Batch:        &StudioVideoBatchCapability{Supported: true, MinCount: 1, MaxCount: 4, Native: false},
		PricingBasis: []string{"duration_seconds", "resolution", "has_video_input", "batch_count"},
		Retention:    &StudioVideoRetention{GeneratedOutput: "upstream_proxy", TemporaryInput: true},
	}
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
