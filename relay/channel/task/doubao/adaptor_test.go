package doubao

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

// Top-level `duration` (int) and `resolution` (string) on the OpenAI Videos
// request shape must reach the upstream Volcano Ark payload — otherwise the
// upstream silently falls back to 5s/720p and bills accordingly.
func TestConvertToRequestPayloadHonoursTopLevelDurationAndResolution(t *testing.T) {
	a := &TaskAdaptor{}

	t.Run("top-level duration and resolution flow through", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:      "doubao-seedance-2-0-260128",
			Prompt:     "切换到面部特写",
			Duration:   4,
			Resolution: "480p",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration, "top-level duration must populate payload")
		require.Equal(t, 4, int(*body.Duration))
		require.Equal(t, "480p", body.Resolution)
	})

	t.Run("metadata still overrides top-level", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:      "doubao-seedance-2-0-260128",
			Prompt:     "p",
			Duration:   4,
			Resolution: "480p",
			Metadata: map[string]any{
				"duration":   8,
				"resolution": "1080p",
			},
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration)
		require.Equal(t, 8, int(*body.Duration))
		require.Equal(t, "1080p", body.Resolution)
	})

	t.Run("seconds string still works as fallback", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:   "doubao-seedance-2-0-260128",
			Prompt:  "p",
			Seconds: "6",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration)
		require.Equal(t, 6, int(*body.Duration))
	})

	t.Run("nothing set leaves duration nil", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:  "doubao-seedance-2-0-260128",
			Prompt: "p",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Nil(t, body.Duration)
		require.Equal(t, "", body.Resolution)
		require.Equal(t, "", body.Ratio)
	})
}

// Top-level `ratio` must reach upstream Volcano Ark; otherwise the API
// silently picks "auto" and the caller's aspect ratio is discarded. See
// https://github.com/NekoAIKan/aikanhub/issues/60.
func TestConvertToRequestPayloadHonoursTopLevelRatio(t *testing.T) {
	a := &TaskAdaptor{}

	t.Run("top-level ratio flows through", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:  "doubao-seedance-2-0-260128",
			Prompt: "p",
			Ratio:  "9:16",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Equal(t, "9:16", body.Ratio)
	})

	t.Run("metadata ratio overrides top-level", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:    "doubao-seedance-2-0-260128",
			Prompt:   "p",
			Ratio:    "9:16",
			Metadata: map[string]any{"ratio": "16:9"},
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Equal(t, "16:9", body.Ratio)
	})

	t.Run("size carries pixel pair into resolution", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:  "doubao-seedance-2-0-260128",
			Prompt: "p",
			Size:   "1280x720",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Equal(t, "1280x720", body.Resolution)
		require.Equal(t, "", body.Ratio)
	})

	t.Run("size carries aspect ratio into ratio", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:  "doubao-seedance-2-0-260128",
			Prompt: "p",
			Size:   "9:16",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Equal(t, "9:16", body.Ratio)
		require.Equal(t, "", body.Resolution)
	})

	t.Run("explicit resolution wins over size", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:      "doubao-seedance-2-0-260128",
			Prompt:     "p",
			Resolution: "480p",
			Size:       "1920x1080",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Equal(t, "480p", body.Resolution)
	})

	t.Run("explicit ratio wins over size ratio", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:  "doubao-seedance-2-0-260128",
			Prompt: "p",
			Ratio:  "16:9",
			Size:   "9:16",
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Equal(t, "16:9", body.Ratio)
	})
}
