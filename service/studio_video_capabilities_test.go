package service

import "testing"

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
	if model.Batch == nil || !model.Batch.Supported || model.Batch.MaxCount != 4 {
		t.Fatalf("unexpected batch capability: %#v", model.Batch)
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
