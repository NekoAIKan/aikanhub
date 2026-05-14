package doubao

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
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

func TestValidateRequestAndSetActionNormalizesSeedanceRootParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	raw := []byte(`{
		"model": "doubao-seedance-2-0-260128",
		"content": [
			{"type": "text", "text": "切换到面部特写"},
			{"type": "image_url", "image_url": {"url": "https://example.com/first.png"}, "role": "first_frame"},
			{"type": "video_url", "video_url": {"url": "https://example.com/ref.mp4"}, "role": "reference_video"},
			{"type": "audio_url", "audio_url": {"url": "https://example.com/ref.mp3"}, "role": "reference_audio"}
		],
		"size": "720x1280",
		"duration": 4,
		"aspect_ratio": "1:1",
		"callback_url": "https://example.com/callback",
		"service_tier": "default",
		"execution_expires_after": 3600,
		"generate_audio": false,
		"return_last_frame": false,
		"draft": false,
		"tools": [{"type": "web_search"}],
		"frames": 0,
		"camera_fixed": false,
		"watermark": false,
		"seed": 0,
		"safety_identifier": "user-hash",
		"metadata": {
			"ratio": "9:16"
		}
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info)
	require.Nil(t, taskErr)

	req, err := relaycommon.GetTaskRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, "切换到面部特写", req.Prompt)

	body, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
	require.NoError(t, err)
	require.Equal(t, "720p", body.Resolution)
	require.Equal(t, "9:16", body.Ratio)
	require.NotNil(t, body.Duration)
	require.Equal(t, 4, int(*body.Duration))
	require.Equal(t, "https://example.com/callback", body.CallbackURL)
	require.Equal(t, "default", body.ServiceTier)
	require.NotNil(t, body.ExecutionExpiresAfter)
	require.Equal(t, 3600, int(*body.ExecutionExpiresAfter))
	require.NotNil(t, body.GenerateAudio)
	require.False(t, bool(*body.GenerateAudio))
	require.NotNil(t, body.ReturnLastFrame)
	require.False(t, bool(*body.ReturnLastFrame))
	require.NotNil(t, body.Draft)
	require.False(t, bool(*body.Draft))
	require.Len(t, body.Tools, 1)
	require.Equal(t, "web_search", body.Tools[0].Type)
	require.NotNil(t, body.Frames)
	require.Equal(t, 0, int(*body.Frames))
	require.NotNil(t, body.CameraFixed)
	require.False(t, bool(*body.CameraFixed))
	require.NotNil(t, body.Watermark)
	require.False(t, bool(*body.Watermark))
	require.NotNil(t, body.Seed)
	require.Equal(t, 0, int(*body.Seed))
	require.Equal(t, "user-hash", body.SafetyIdentifier)

	require.Len(t, body.Content, 4)
	require.Equal(t, "image_url", body.Content[0].Type)
	require.Equal(t, "first_frame", body.Content[0].Role)
	require.Equal(t, "https://example.com/first.png", body.Content[0].ImageURL.URL)
	require.Equal(t, "video_url", body.Content[1].Type)
	require.Equal(t, "reference_video", body.Content[1].Role)
	require.Equal(t, "https://example.com/ref.mp4", body.Content[1].VideoURL.URL)
	require.Equal(t, "audio_url", body.Content[2].Type)
	require.Equal(t, "reference_audio", body.Content[2].Role)
	require.Equal(t, "https://example.com/ref.mp3", body.Content[2].AudioURL.URL)
	require.Equal(t, "text", body.Content[3].Type)
	require.Equal(t, "切换到面部特写", body.Content[3].Text)
}

func TestVolcArkOfficialSubmitRouteKeepsSeedanceFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	raw := []byte(`{
		"model": "doubao-seedance-1-5-pro-251215",
		"content": [
			{"type": "text", "text": "生成正式视频"},
			{"type": "draft_task", "draft_task": {"id": "task_draft_123"}}
		],
		"callback_url": "https://example.com/callback",
		"return_last_frame": true,
		"service_tier": "default",
		"execution_expires_after": 3600,
		"generate_audio": false,
		"draft": false,
		"tools": [{"type": "web_search"}],
		"resolution": "720p",
		"ratio": "16:9",
		"duration": 5,
		"frames": 0,
		"seed": 0,
		"camera_fixed": false,
		"watermark": false,
		"safety_identifier": "user-hash"
	}`)

	var upstream requestPayload
	router := gin.New()
	router.POST("/api/v3/contents/generations/tasks", middleware.VolcArkSubmitConvert(), func(c *gin.Context) {
		info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
		taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
		require.Nil(t, taskErr)

		req, err := relaycommon.GetTaskRequest(c)
		require.NoError(t, err)
		body, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
		require.NoError(t, err)
		upstream = *body

		c.JSON(http.StatusOK, gin.H{"task_id": "task_public_123"})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v3/contents/generations/tasks", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"id":"task_public_123"}`, recorder.Body.String())
	require.Equal(t, "doubao-seedance-1-5-pro-251215", upstream.Model)
	require.Equal(t, "https://example.com/callback", upstream.CallbackURL)
	require.NotNil(t, upstream.ReturnLastFrame)
	require.True(t, bool(*upstream.ReturnLastFrame))
	require.Equal(t, "default", upstream.ServiceTier)
	require.NotNil(t, upstream.ExecutionExpiresAfter)
	require.Equal(t, 3600, int(*upstream.ExecutionExpiresAfter))
	require.NotNil(t, upstream.GenerateAudio)
	require.False(t, bool(*upstream.GenerateAudio))
	require.NotNil(t, upstream.Draft)
	require.False(t, bool(*upstream.Draft))
	require.Len(t, upstream.Tools, 1)
	require.Equal(t, "web_search", upstream.Tools[0].Type)
	require.Equal(t, "720p", upstream.Resolution)
	require.Equal(t, "16:9", upstream.Ratio)
	require.NotNil(t, upstream.Duration)
	require.Equal(t, 5, int(*upstream.Duration))
	require.NotNil(t, upstream.Frames)
	require.Equal(t, 0, int(*upstream.Frames))
	require.NotNil(t, upstream.Seed)
	require.Equal(t, 0, int(*upstream.Seed))
	require.NotNil(t, upstream.CameraFixed)
	require.False(t, bool(*upstream.CameraFixed))
	require.NotNil(t, upstream.Watermark)
	require.False(t, bool(*upstream.Watermark))
	require.Equal(t, "user-hash", upstream.SafetyIdentifier)
	require.Len(t, upstream.Content, 2)
	require.Equal(t, "draft_task", upstream.Content[0].Type)
	require.Equal(t, "task_draft_123", upstream.Content[0].DraftTask["id"])
	require.Equal(t, "text", upstream.Content[1].Type)
	require.Equal(t, "生成正式视频", upstream.Content[1].Text)
}

func TestValidateRequestAndSetActionAllowsSeedanceMediaOnlyContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	raw := []byte(`{
		"model": "doubao-seedance-2-0-260128",
		"content": [
			{"type": "image_url", "image_url": {"url": "https://example.com/first.png"}, "role": "first_frame"}
		],
		"resolution": "720p",
		"ratio": "16:9"
	}`)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, info)
	require.Nil(t, taskErr)

	req, err := relaycommon.GetTaskRequest(ctx)
	require.NoError(t, err)
	body, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
	require.NoError(t, err)
	require.Len(t, body.Content, 1)
	require.Equal(t, "image_url", body.Content[0].Type)
}

func TestConvertToRequestPayloadTranslatesSizeToSeedanceResolutionAndRatio(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantResolution string
		wantRatio      string
	}{
		{
			name:           "landscape 720p pixel size",
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"p","size":"1280x720"}`,
			wantResolution: "720p",
			wantRatio:      "16:9",
		},
		{
			name:           "portrait 720p pixel size",
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"p","size":"720x1280"}`,
			wantResolution: "720p",
			wantRatio:      "9:16",
		},
		{
			name:           "landscape 1080p pixel size",
			body:           `{"model":"doubao-seedance-2-0-260128","prompt":"p","size":"1920x1080"}`,
			wantResolution: "1080p",
			wantRatio:      "16:9",
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
			normalizeSeedanceTaskRequest(&req, mustObject(t, tt.body))

			body, err := (&TaskAdaptor{}).convertToRequestPayload(&req)
			require.NoError(t, err)
			require.Equal(t, tt.wantResolution, body.Resolution)
			require.Equal(t, tt.wantRatio, body.Ratio)
		})
	}
}

func mustObject(t *testing.T, body string) map[string]any {
	t.Helper()
	var raw map[string]any
	require.NoError(t, common.Unmarshal([]byte(body), &raw))
	return raw
}
