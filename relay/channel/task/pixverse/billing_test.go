package pixverse

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
	"github.com/stretchr/testify/require"
)

func testProfile() videobilling.VideoBillingProfile {
	return videobilling.VideoBillingProfile{
		Mode:                    videobilling.ModePerSecond,
		PricePerSecond:          0.04,
		PricePerSecondWithAudio: 0.05,
		FallbackFPS:             24,
		FallbackWidth:           960,
		FallbackHeight:          540,
		FallbackDurationSeconds: 5,
		ConservativeMultiplier:  1.0,
		ResolutionAliases: map[string]videobilling.VideoResolution{
			"360p":  {Width: 640, Height: 360},
			"540p":  {Width: 960, Height: 540},
			"720p":  {Width: 1280, Height: 720},
			"1080p": {Width: 1920, Height: 1080},
		},
	}
}

func TestExtractRequestBillingInput_TopLevelDurationAndQuality(t *testing.T) {
	got := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{
		Duration: 8,
		Metadata: map[string]any{"quality": "720p"},
	}, testProfile(), 1)

	require.Equal(t, service.VideoBillingInput{
		OutputSeconds: 8,
		Width:         1280,
		Height:        720,
		Resolution:    "720p",
		GroupRatio:    1,
	}, got)
}

func TestExtractRequestBillingInput_AudioFlag(t *testing.T) {
	got := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{
		Duration: 5,
		Metadata: map[string]any{
			"quality":               "1080p",
			"generate_audio_switch": true,
		},
	}, testProfile(), 1)

	require.True(t, got.HasAudioInput)
	require.Equal(t, "1080p", got.Resolution)
	require.Equal(t, 1920, got.Width)
}

func TestExtractRequestBillingInput_GenerateAudioAlias(t *testing.T) {
	// Some SDK versions use the shorter `generate_audio` field name.
	got := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{
		Duration: 5,
		Metadata: map[string]any{"quality": "540p", "generate_audio": "true"},
	}, testProfile(), 1)
	require.True(t, got.HasAudioInput)
}

func TestExtractRequestBillingInput_FallbacksWhenMissing(t *testing.T) {
	// Empty request: duration falls back to profile, dimensions to default.
	got := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{}, testProfile(), 1)
	require.Equal(t, 5, got.OutputSeconds)
	require.Equal(t, 960, got.Width)
	require.Equal(t, 540, got.Height)
	require.False(t, got.HasAudioInput)
}

func TestExtractRequestBillingInput_TopLevelResolutionWins(t *testing.T) {
	got := ExtractRequestBillingInput(relaycommon.TaskSubmitReq{
		Duration:   5,
		Resolution: "720p",
	}, testProfile(), 1)
	require.Equal(t, "720p", got.Resolution)
	require.Equal(t, 1280, got.Width)
}
