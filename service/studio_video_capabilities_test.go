package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestDescribeStudioVideoModelSeedanceCapabilities(t *testing.T) {
	model, ok := DescribeStudioVideoModel("doubao-seedance-2-0-260128")
	if !ok {
		t.Fatal("expected seedance model to be recognized")
	}

	if model.ID != "doubao-seedance-2-0-260128" {
		t.Fatalf("unexpected model id: %s", model.ID)
	}
	if model.Provider != "doubao" {
		t.Fatalf("unexpected provider: %s", model.Provider)
	}
	if len(model.Versions) != 1 || model.Versions[0].ID != "default" || model.Versions[0].Model != "doubao-seedance-2-0-260128" {
		t.Fatalf("unexpected versions: %#v", model.Versions)
	}
	if !model.Inputs.ReferenceVideos.Supported || model.Inputs.ReferenceVideos.Max != 3 {
		t.Fatalf("expected reference video capability, got %#v", model.Inputs.ReferenceVideos)
	}
	if !model.Inputs.ReferenceAudios.Supported || !model.Inputs.ReferenceAudios.RequiresMediaInput {
		t.Fatalf("expected gated reference audio capability, got %#v", model.Inputs.ReferenceAudios)
	}
	if model.SchemaVersion != "2026-05-21" {
		t.Fatalf("unexpected schema version: %s", model.SchemaVersion)
	}
	if !containsInt(model.Settings.Durations, 4) || !containsInt(model.Settings.Durations, 15) {
		t.Fatalf("expected Seedance standard duration range 4-15, got %#v", model.Settings.Durations)
	}
	if !containsString(model.Settings.Ratios, "adaptive") {
		t.Fatalf("expected adaptive ratio, got %#v", model.Settings.Ratios)
	}
	if !modeSupported(model.Modes, "image_first_last_frame") || !modeSupported(model.Modes, "multimodal_reference") {
		t.Fatalf("expected first/last and multimodal modes, got %#v", model.Modes)
	}
	if !modeSupported(model.Modes, "edit_video") || !modeSupported(model.Modes, "extend_video") || !modeSupported(model.Modes, "web_search") {
		t.Fatalf("expected full Seedance 2.0 modes, got %#v", model.Modes)
	}
	if model.Batch == nil || !model.Batch.Supported || model.Batch.MaxCount != 4 {
		t.Fatalf("unexpected batch capability: %#v", model.Batch)
	}
	if model.Selection == nil || model.Selection.RegionAliases["doubao-seedance-2-0-260128"] != "seedance-2.0-global" {
		t.Fatalf("expected Seedance model selection aliases, got %#v", model.Selection)
	}
	if !parameterAdvertised(model.Parameters, "metadata.tools") || !parameterAdvertised(model.Parameters, "metadata.generate_audio") {
		t.Fatalf("expected Seedance docs-derived parameters, got %#v", model.Parameters)
	}
}

func TestDescribeStudioVideoModelSeedanceFastCanonicalizesVersion(t *testing.T) {
	model, ok := DescribeStudioVideoModel("doubao-seedance-2-0-Fast-260128")
	if !ok {
		t.Fatal("expected seedance fast model to be recognized")
	}

	if model.ID != "doubao-seedance-2-0-260128" {
		t.Fatalf("unexpected canonical id: %s", model.ID)
	}
	if len(model.Versions) != 1 || model.Versions[0].ID != "fast" || model.Versions[0].Model != "doubao-seedance-2-0-Fast-260128" {
		t.Fatalf("unexpected versions: %#v", model.Versions)
	}
	if containsString(model.Settings.Resolutions, "1080p") {
		t.Fatalf("fast version should not advertise 1080p, got %#v", model.Settings.Resolutions)
	}
	if containsInt(model.Settings.Durations, 13) {
		t.Fatalf("fast version should cap duration at 12s, got %#v", model.Settings.Durations)
	}
	if modeSupported(model.Modes, "web_search") || modeSupported(model.Modes, "edit_video") || modeSupported(model.Modes, "extend_video") {
		t.Fatalf("fast version should not advertise advanced standard-only modes, got %#v", model.Modes)
	}
	if !parameterAdvertised(model.Parameters, "metadata.content") {
		t.Fatalf("fast version should still advertise shared metadata.content parameter, got %#v", model.Parameters)
	}
}

func TestDescribeStudioVideoModelPixverseCapabilities(t *testing.T) {
	model, ok := DescribeStudioVideoModel("pixverse-v4.5-720p-8s")
	if !ok {
		t.Fatal("expected pixverse model to be recognized")
	}

	if model.Provider != "pixverse" {
		t.Fatalf("unexpected provider: %s", model.Provider)
	}
	if model.Selection == nil || !model.Selection.PinnedSpecModelIDs || !model.Selection.QualityLockedByModelID || !model.Selection.DurationLockedByModelID {
		t.Fatalf("expected pinned Pixverse selection metadata, got %#v", model.Selection)
	}
	if !containsString(model.Settings.Resolutions, "720p") || len(model.Settings.Resolutions) != 1 {
		t.Fatalf("expected pinned 720p resolution, got %#v", model.Settings.Resolutions)
	}
	if !containsInt(model.Settings.Durations, 8) || len(model.Settings.Durations) != 1 {
		t.Fatalf("expected pinned 8s duration, got %#v", model.Settings.Durations)
	}
	if model.Inputs.ReferenceVideos.Supported || model.Inputs.ReferenceAudios.Supported {
		t.Fatalf("pixverse should not advertise video/audio references, got videos=%#v audios=%#v", model.Inputs.ReferenceVideos, model.Inputs.ReferenceAudios)
	}
	if !modeSupported(model.Modes, "image_first_last_frame") || !modeSupported(model.Modes, "multimodal_reference") {
		t.Fatalf("expected transition and fusion modes, got %#v", model.Modes)
	}
	if !parameterAdvertised(model.Parameters, "metadata.image_references") || !parameterAdvertised(model.Parameters, "metadata.generate_audio_switch") {
		t.Fatalf("expected Pixverse-specific parameters, got %#v", model.Parameters)
	}
}

func TestStudioBootstrapContractDoesNotExposeTokenSecrets(t *testing.T) {
	originalSessionSecret := common.SessionSecret
	t.Cleanup(func() {
		common.SessionSecret = originalSessionSecret
	})
	common.SessionSecret = "studio-bootstrap-test-secret"
	data := newStudioBootstrapData(42, "creator", "default", 1, &model.Token{
		Status:          common.TokenStatusEnabled,
		Source:          model.TokenSourceStudio,
		ManagedBySystem: true,
		Key:             "sk-hidden",
	})

	if data.Session.ID != 42 || data.Auth.Mode != "kittyvibe-session" || data.Auth.UserHeader != "New-Api-User" {
		t.Fatalf("unexpected bootstrap auth/session data: %#v", data)
	}
	if len(data.CredentialProfiles) != 1 || data.CredentialProfiles[0].Status != "ready" || data.CredentialProfiles[0].SecretExposed {
		t.Fatalf("unexpected credential profiles: %#v", data.CredentialProfiles)
	}
	if !VerifyStudioCSRFToken(data.Auth.CSRFToken, 42, time.Now()) {
		t.Fatalf("expected bootstrap csrf token to verify")
	}

	payload, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal bootstrap: %v", err)
	}
	serialized := string(payload)
	for _, forbidden := range []string{"sk-hidden", `"key"`, `"apiKey"`, `"secret"`} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("bootstrap response exposed forbidden token material %q in %s", forbidden, serialized)
		}
	}
}

func TestMergeStudioVideoVersionsDeduplicates(t *testing.T) {
	versions := mergeStudioVideoVersions(
		[]StudioVideoModelVersion{{ID: "default", Label: "Standard", Model: "seedance-standard"}},
		[]StudioVideoModelVersion{
			{ID: "default", Label: "Standard", Model: "seedance-standard"},
			{ID: "fast", Label: "Fast", Model: "seedance-fast"},
		},
	)

	if len(versions) != 2 {
		t.Fatalf("expected two unique versions, got %#v", versions)
	}
	if versions[1].ID != "fast" {
		t.Fatalf("expected fast version to be appended, got %#v", versions)
	}
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func modeSupported(modes []StudioVideoMode, id string) bool {
	for _, mode := range modes {
		if mode.ID == id {
			return mode.Supported
		}
	}
	return false
}

func parameterAdvertised(parameters []StudioVideoParameter, name string) bool {
	for _, parameter := range parameters {
		if parameter.Name == name {
			return true
		}
	}
	return false
}
