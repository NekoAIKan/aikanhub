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
	})
}
