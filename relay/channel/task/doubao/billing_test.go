package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
	"github.com/stretchr/testify/require"
)

func testProfile() videobilling.VideoBillingProfile {
	return videobilling.VideoBillingProfile{
		Mode:                            videobilling.ModeFormula,
		UnitPrice:                       1,
		FallbackFPS:                     24,
		FallbackWidth:                   1280,
		FallbackHeight:                  720,
		FallbackDurationSeconds:         5,
		UseUpstreamUsage:                true,
		ConservativeMultiplier:          1.25,
		ReferenceConservativeMultiplier: 2,
		DraftMultiplier:                 0.5,
		ResolutionAliases: map[string]videobilling.VideoResolution{
			"720p":     {Width: 1280, Height: 720},
			"1080p":    {Width: 1920, Height: 1080},
			"1280x720": {Width: 1280, Height: 720},
		},
	}
}

func TestExtractRequestBillingInput(t *testing.T) {
	tests := []struct {
		name string
		req  relaycommon.TaskSubmitReq
		want service.VideoBillingInput
	}{
		{
			name: "duration and resolution",
			req: relaycommon.TaskSubmitReq{
				Duration: 5,
				Metadata: map[string]any{
					"resolution": "1080p",
				},
			},
			want: service.VideoBillingInput{OutputSeconds: 5, Width: 1920, Height: 1080, FPS: 24, GroupRatio: 1},
		},
		{
			name: "seconds and size aliases",
			req: relaycommon.TaskSubmitReq{
				Seconds: "6",
				Size:    "1280x720",
			},
			want: service.VideoBillingInput{OutputSeconds: 6, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1},
		},
		{
			name: "metadata seconds draft and content video",
			req: relaycommon.TaskSubmitReq{
				Metadata: map[string]any{
					"seconds": "7",
					"draft":   true,
					"content": []any{
						map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://example.com/in.mp4"}},
					},
				},
			},
			want: service.VideoBillingInput{InputSeconds: 7, OutputSeconds: 7, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1, Draft: true, HasReferenceMedia: true},
		},
		{
			name: "metadata duration generate audio is ignored",
			req: relaycommon.TaskSubmitReq{
				Metadata: map[string]any{
					"duration":       float64(8),
					"generate_audio": true,
				},
			},
			want: service.VideoBillingInput{OutputSeconds: 8, Width: 1280, Height: 720, FPS: 24, GroupRatio: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRequestBillingInput(tt.req, testProfile(), 1)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExtractRequestBillingInputMarksReferenceMedia(t *testing.T) {
	got := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{
		Duration: 5,
		Metadata: map[string]any{
			"content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/in.png"}},
				map[string]any{"type": "video_url", "video_url": map[string]any{"url": "https://example.com/in.mp4"}},
			},
			"resolution": "720p",
		},
	}, testProfile(), 1)

	require.True(t, got.HasReferenceMedia)
	precharge := service.CalculateVideoBilling(testProfile(), got, true)
	require.GreaterOrEqual(t, precharge.Quota, 216000)
	require.GreaterOrEqual(t, precharge.Quota, 162450)
}

func TestExtractRequestBillingInputKeepsPlainPrechargeLegacyValues(t *testing.T) {
	plain720 := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{
		Duration: 5,
		Metadata: map[string]any{
			"resolution": "720p",
		},
	}, testProfile(), 1)
	require.False(t, plain720.HasReferenceMedia)
	require.Equal(t, 67500, service.CalculateVideoBilling(testProfile(), plain720, true).Quota)

	plain1080 := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{
		Duration: 5,
		Metadata: map[string]any{
			"resolution": "1080p",
		},
	}, testProfile(), 1)
	require.False(t, plain1080.HasReferenceMedia)
	require.Equal(t, 151875, service.CalculateVideoBilling(testProfile(), plain1080, true).Quota)
}

func TestExtractResponseBillingInput(t *testing.T) {
	body := []byte(`{
		"id": "task-upstream",
		"status": "succeeded",
		"resolution": "1080p",
		"duration": 9,
		"framespersecond": 30,
		"usage": {"total_tokens": 12345}
	}`)

	got, err := ExtractResponseBillingInput(body, testProfile(), 1)
	require.NoError(t, err)
	require.Equal(t, service.VideoBillingInput{
		OutputSeconds:       9,
		Width:               1920,
		Height:              1080,
		FPS:                 30,
		GroupRatio:          1,
		UpstreamTotalTokens: 12345,
	}, got)
}

func TestAdjustBillingOnCompleteUsesProfileContextAndResponse(t *testing.T) {
	profile := testProfile()
	task := &model.Task{
		Quota: 1,
		Group: "default",
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				BillingMode:     videobilling.ModeFormula,
				BillingProfile:  "doubao-seedance-test",
				GroupRatio:      1,
				OriginModelName: "doubao-seedance-test",
				VideoParams: map[string]any{
					"input_seconds":  float64(10),
					"output_seconds": float64(5),
					"width":          float64(1280),
					"height":         float64(720),
					"fps":            float64(24),
				},
			},
		},
	}
	videoBillingProfilesForTest(t, map[string]videobilling.VideoBillingProfile{
		"doubao-seedance-test": profile,
	})

	task.Data = []byte(`{"status":"succeeded","usage":{"total_tokens":1000},"resolution":"1080p","duration":9,"framespersecond":30}`)
	got := (&TaskAdaptor{}).AdjustBillingOnComplete(task, &relaycommon.TaskInfo{TotalTokens: 1000})
	require.Equal(t, service.CalculateVideoBilling(profile, service.VideoBillingInput{
		InputSeconds:        10,
		OutputSeconds:       9,
		Width:               1920,
		Height:              1080,
		FPS:                 30,
		GroupRatio:          1,
		UpstreamTotalTokens: 1000,
	}, false).Quota, got)
}

func videoBillingProfilesForTest(t *testing.T, profiles map[string]videobilling.VideoBillingProfile) {
	t.Helper()
	b, err := common.Marshal(profiles)
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"video_billing_setting.profiles": string(b),
	}))
}
