package doubao

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newJSONContext builds a gin.Context with the given JSON body, suitable
// for driving relay-layer parsers (UnmarshalBodyReusable, etc.) in tests.
// Each test gets its own context — body is read multiple times by both
// ValidateBasicTaskRequest and the fold helper, and gin handles the
// buffering internally via UnmarshalBodyReusable.
func newJSONContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/v1/video/generations",
		bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

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

// 智能时长: the documented `duration:-1` "let the model decide" sentinel
// must reach upstream. The old `> 0` guard in convertToRequestPayload
// swallowed it on the top-level (OpenAI-shape) entry, so Volc fell back
// to its fixed default and adaptive duration silently didn't work.
// See https://github.com/NekoAIKan/aikanhub/issues/68.
func TestConvertToRequestPayloadForwardsAdaptiveDurationSentinel(t *testing.T) {
	a := &TaskAdaptor{}

	t.Run("top-level duration -1 (int) reaches upstream", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{Model: "m", Prompt: "p", Duration: -1}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration, "adaptive -1 must not be dropped")
		require.Equal(t, -1, int(*body.Duration))
	})

	t.Run("top-level duration \"-1\" (string) reaches upstream", func(t *testing.T) {
		var req relaycommon.TaskSubmitReq
		require.NoError(t, common.Unmarshal(
			[]byte(`{"model":"m","prompt":"p","duration":"-1"}`), &req))
		body, err := a.convertToRequestPayload(&req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration)
		require.Equal(t, -1, int(*body.Duration))
	})

	t.Run("positive duration still works (no regression)", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{Model: "m", Prompt: "p", Duration: 8}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration)
		require.Equal(t, 8, int(*body.Duration))
	})

	t.Run("duration 0 / unset is still omitted (upstream default)", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{Model: "m", Prompt: "p", Duration: 0}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.Nil(t, body.Duration)
	})

	t.Run("seconds=\"-1\" fallback also forwards the sentinel", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{Model: "m", Prompt: "p", Seconds: "-1"}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration)
		require.Equal(t, -1, int(*body.Duration))
	})

	t.Run("metadata duration -1 path keeps working", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model: "m", Prompt: "p",
			Metadata: map[string]any{"duration": -1},
		}
		body, err := a.convertToRequestPayload(req)
		require.NoError(t, err)
		require.NotNil(t, body.Duration)
		require.Equal(t, -1, int(*body.Duration))
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

	t.Run("top-level aspect_ratio alias flows through", func(t *testing.T) {
		var req relaycommon.TaskSubmitReq
		require.NoError(t, common.Unmarshal([]byte(`{
			"model": "doubao-seedance-2-0-260128",
			"prompt": "p",
			"aspect_ratio": "9:16"
		}`), &req))
		body, err := a.convertToRequestPayload(&req)
		require.NoError(t, err)
		require.Equal(t, "9:16", body.Ratio)
	})

	t.Run("ratio wins over aspect_ratio alias", func(t *testing.T) {
		req := &relaycommon.TaskSubmitReq{
			Model:       "doubao-seedance-2-0-260128",
			Prompt:      "p",
			Ratio:       "16:9",
			AspectRatio: "9:16",
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

func TestConvertToRequestPayloadKeepsSizeHandlingLimitedToCurrentBug(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantResolution string
		wantRatio      string
	}{
		{
			name:           "pixel size remains resolution fallback",
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"p","size":"1280x720"}`,
			wantResolution: "1280x720",
			wantRatio:      "",
		},
		{
			name:           "aspect ratio size alias",
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"p","size":"9:16"}`,
			wantResolution: "",
			wantRatio:      "9:16",
		},
		{
			name:           "aspect_ratio alias",
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"p","aspect_ratio":"3:4"}`,
			wantResolution: "",
			wantRatio:      "3:4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req relaycommon.TaskSubmitReq
			require.NoError(t, common.Unmarshal([]byte(tt.body), &req))

			body, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
			require.NoError(t, err)
			require.Equal(t, tt.wantResolution, body.Resolution)
			require.Equal(t, tt.wantRatio, body.Ratio)
		})
	}
}

// Volc Ark native root fields (seed, watermark, camera_fixed,
// return_last_frame, ...) on an OpenAI-shape `/v1/videos` request body
// must reach upstream. TaskSubmitReq doesn't declare them, so without
// foldVolcRootFieldsIntoMetadata they're silently dropped during JSON
// parse and the upstream substitutes its defaults — see
// https://github.com/NekoAIKan/aikanhub/issues/63.
func TestFoldVolcRootFieldsIntoMetadata(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantKey   string
		wantValue interface{} // matched with require.EqualValues
	}{
		{
			name:      "seed",
			body:      `{"model":"m","prompt":"p","seed":12345}`,
			wantKey:   "seed",
			wantValue: 12345,
		},
		{
			name:      "watermark",
			body:      `{"model":"m","prompt":"p","watermark":false}`,
			wantKey:   "watermark",
			wantValue: false,
		},
		{
			name:      "camera_fixed",
			body:      `{"model":"m","prompt":"p","camera_fixed":true}`,
			wantKey:   "camera_fixed",
			wantValue: true,
		},
		{
			name:      "return_last_frame",
			body:      `{"model":"m","prompt":"p","return_last_frame":true}`,
			wantKey:   "return_last_frame",
			wantValue: true,
		},
		{
			name:      "callback_url",
			body:      `{"model":"m","prompt":"p","callback_url":"https://example.com/cb"}`,
			wantKey:   "callback_url",
			wantValue: "https://example.com/cb",
		},
		{
			name:      "service_tier",
			body:      `{"model":"m","prompt":"p","service_tier":"priority"}`,
			wantKey:   "service_tier",
			wantValue: "priority",
		},
		{
			name:      "execution_expires_after",
			body:      `{"model":"m","prompt":"p","execution_expires_after":172800}`,
			wantKey:   "execution_expires_after",
			wantValue: 172800,
		},
		{
			name:      "generate_audio",
			body:      `{"model":"m","prompt":"p","generate_audio":true}`,
			wantKey:   "generate_audio",
			wantValue: true,
		},
		{
			name:      "draft",
			body:      `{"model":"m","prompt":"p","draft":true}`,
			wantKey:   "draft",
			wantValue: true,
		},
		{
			name:      "frames",
			body:      `{"model":"m","prompt":"p","frames":24}`,
			wantKey:   "frames",
			wantValue: 24,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := newJSONContext(tt.body)
			// First seed the context with a parsed task request, the way
			// ValidateBasicTaskRequest does in production. We can't call
			// ValidateBasicTaskRequest directly here because it depends
			// on info.Action and other RelayInfo state not relevant to
			// the fold logic; mimicking its store call is sufficient.
			var req relaycommon.TaskSubmitReq
			require.NoError(t, common.UnmarshalBodyReusable(c, &req))
			c.Set("task_request", req)

			require.NoError(t, foldVolcRootFieldsIntoMetadata(c))

			out, err := relaycommon.GetTaskRequest(c)
			require.NoError(t, err)
			require.NotNil(t, out.Metadata, "fold must allocate metadata when nil")
			got, present := out.Metadata[tt.wantKey]
			require.True(t, present, "key %q must be folded into metadata", tt.wantKey)
			require.EqualValues(t, tt.wantValue, got)
		})
	}
}

// Metadata-set values take precedence over root-level for the same key.
// Without this, a caller who explicitly placed `metadata.seed=2` would
// see it silently overwritten by an inherited / cached `seed=1` at root.
func TestFoldVolcRootFieldsIntoMetadata_MetadataWins(t *testing.T) {
	body := `{
		"model": "m",
		"prompt": "p",
		"seed": 1,
		"watermark": false,
		"metadata": { "seed": 2, "watermark": true }
	}`
	c := newJSONContext(body)
	var req relaycommon.TaskSubmitReq
	require.NoError(t, common.UnmarshalBodyReusable(c, &req))
	c.Set("task_request", req)

	require.NoError(t, foldVolcRootFieldsIntoMetadata(c))

	out, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	require.EqualValues(t, 2, out.Metadata["seed"])
	require.EqualValues(t, true, out.Metadata["watermark"])
}

// End-to-end: after fold, convertToRequestPayload must populate the
// matching requestPayload field via UnmarshalMetadata. Drives every
// supported field through the full pipeline so the fold + UnmarshalMetadata
// + struct-tag wiring stay in sync.
func TestConvertToRequestPayloadHonoursFoldedVolcRootFields(t *testing.T) {
	body := `{
		"model": "doubao-seedance-2-0-260128",
		"prompt": "p",
		"seed": 12345,
		"watermark": false,
		"camera_fixed": true,
		"return_last_frame": true,
		"service_tier": "priority",
		"execution_expires_after": 172800,
		"generate_audio": false,
		"draft": true,
		"frames": 24,
		"tools": [{"type":"web_search"}]
	}`
	c := newJSONContext(body)
	var req relaycommon.TaskSubmitReq
	require.NoError(t, common.UnmarshalBodyReusable(c, &req))
	c.Set("task_request", req)
	require.NoError(t, foldVolcRootFieldsIntoMetadata(c))

	out, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	payload, err := (&TaskAdaptor{}).convertToRequestPayload(&out)
	require.NoError(t, err)

	require.NotNil(t, payload.Seed)
	require.Equal(t, 12345, int(*payload.Seed))
	require.NotNil(t, payload.Watermark)
	require.Equal(t, false, bool(*payload.Watermark))
	require.NotNil(t, payload.CameraFixed)
	require.Equal(t, true, bool(*payload.CameraFixed))
	require.NotNil(t, payload.ReturnLastFrame)
	require.Equal(t, true, bool(*payload.ReturnLastFrame))
	require.Equal(t, "priority", payload.ServiceTier)
	require.NotNil(t, payload.ExecutionExpiresAfter)
	require.Equal(t, 172800, int(*payload.ExecutionExpiresAfter))
	require.NotNil(t, payload.GenerateAudio)
	require.Equal(t, false, bool(*payload.GenerateAudio))
	require.NotNil(t, payload.Draft)
	require.Equal(t, true, bool(*payload.Draft))
	require.NotNil(t, payload.Frames)
	require.Equal(t, 24, int(*payload.Frames))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "web_search", payload.Tools[0].Type)
}
