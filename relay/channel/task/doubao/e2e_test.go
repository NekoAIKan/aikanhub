package doubao

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type seedanceFixture struct {
	Name                         string          `json:"name"`
	Scenario                     string          `json:"scenario"`
	Sanitized                    bool            `json:"sanitized"`
	RequestPayload               requestPayload  `json:"request_payload"`
	SubmitResponsePayload        responsePayload `json:"submit_response_payload"`
	FinalResponsePayload         responseTask    `json:"final_response_payload"`
	TerminalErrorResponsePayload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	} `json:"terminal_error_response_payload"`
	BillingFields struct {
		Model            string `json:"model"`
		Resolution       string `json:"resolution"`
		Duration         int    `json:"duration"`
		FPS              *int   `json:"fps"`
		CompletionTokens *int   `json:"completion_tokens"`
		TotalTokens      *int   `json:"total_tokens"`
	} `json:"billing_fields"`
	VideoArtifact struct {
		Kind   string  `json:"kind"`
		Path   *string `json:"path"`
		URL    *string `json:"url"`
		Reason string  `json:"reason"`
	} `json:"video_artifact"`
}

func TestSeedanceFixtureMockReplaySuccesses(t *testing.T) {
	for _, name := range []string{"seedance_720p_5s", "seedance_1080p_5s"} {
		t.Run(name, func(t *testing.T) {
			fixture := loadSeedanceFixture(t, name)
			server := seedanceFixtureServer(t, fixture)
			defer server.Close()

			adaptor := &TaskAdaptor{apiKey: "fixture-test-key", baseURL: server.URL}
			info := &relaycommon.RelayInfo{
				RelayMode:       relayconstant.RelayModeVideoSubmit,
				OriginModelName: fixture.RequestPayload.Model,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: fixture.RequestPayload.Model,
				},
				TaskRelayInfo: &relaycommon.TaskRelayInfo{
					PublicTaskID: "task_fixture_" + name,
				},
			}

			requestBody := mustMarshalReader(t, fixture.RequestPayload)
			submitURL, err := adaptor.BuildRequestURL(info)
			require.NoError(t, err)
			submitReq, err := http.NewRequest(http.MethodPost, submitURL, requestBody)
			require.NoError(t, err)
			require.NoError(t, adaptor.BuildRequestHeader(nil, submitReq, info))
			submitResp, err := http.DefaultClient.Do(submitReq)
			require.NoError(t, err)

			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			taskID, taskData, taskErr := adaptor.DoResponse(ctx, submitResp, info)
			require.Nil(t, taskErr)
			require.Equal(t, fixture.SubmitResponsePayload.ID, taskID)
			require.Contains(t, string(taskData), fixture.SubmitResponsePayload.ID)

			pollReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/v3/contents/generations/tasks/"+taskID, nil)
			require.NoError(t, err)
			pollReq.Header.Set("Authorization", "Bearer fixture-test-key")
			pollResp, err := http.DefaultClient.Do(pollReq)
			require.NoError(t, err)
			defer pollResp.Body.Close()
			pollBody, err := io.ReadAll(pollResp.Body)
			require.NoError(t, err)

			taskInfo, err := adaptor.ParseTaskResult(pollBody)
			require.NoError(t, err)
			require.Equal(t, string(model.TaskStatusSuccess), taskInfo.Status)
			require.Equal(t, "100%", taskInfo.Progress)
			require.Equal(t, *fixture.BillingFields.TotalTokens, taskInfo.TotalTokens)
			require.Equal(t, *fixture.BillingFields.CompletionTokens, taskInfo.CompletionTokens)
			require.Equal(t, *fixture.VideoArtifact.URL, taskInfo.Url)

			billingInput, err := ExtractResponseBillingInput(pollBody, testProfile(), 1)
			require.NoError(t, err)
			require.Equal(t, fixture.BillingFields.Duration, billingInput.OutputSeconds)
			require.Equal(t, *fixture.BillingFields.FPS, billingInput.FPS)
			require.Equal(t, *fixture.BillingFields.TotalTokens, billingInput.UpstreamTotalTokens)

			result := service.CalculateVideoBilling(testProfile(), billingInput, false)
			require.Equal(t, service.VideoBillingBasisUpstreamUsage, result.Basis)
			require.Equal(t, *fixture.BillingFields.TotalTokens, result.Tokens)
		})
	}
}

func TestSeedanceFixtureValidationFailureSettlementInput(t *testing.T) {
	fixture := loadSeedanceFixture(t, "seedance_failure")
	require.Equal(t, "submit_validation_failure", fixture.Scenario)
	require.True(t, fixture.Sanitized)
	require.Equal(t, "InvalidParameter", fixture.TerminalErrorResponsePayload.Error.Code)
	require.Equal(t, "BadRequest", fixture.TerminalErrorResponsePayload.Error.Type)
	require.Contains(t, fixture.TerminalErrorResponsePayload.Error.Message, "resolution")
	require.Equal(t, "none", fixture.VideoArtifact.Kind)

	taskInfo := relaycommon.TaskInfo{
		Status: string(model.TaskStatusFailure),
		Reason: fixture.TerminalErrorResponsePayload.Error.Message,
	}
	require.Equal(t, string(model.TaskStatusFailure), taskInfo.Status)
	require.Zero(t, taskInfo.TotalTokens)
	require.Zero(t, taskInfo.CompletionTokens)
}

func TestSeedanceFixturesAreSanitizedAndParseBillingFields(t *testing.T) {
	for _, name := range []string{"seedance_720p_5s", "seedance_1080p_5s", "seedance_failure"} {
		t.Run(name, func(t *testing.T) {
			fixture := loadSeedanceFixture(t, name)
			raw := readFixtureBytes(t, name)
			require.NotContains(t, string(raw), "ark-")
			require.NotContains(t, string(raw), "Authorization")
			require.True(t, fixture.Sanitized)
			require.Equal(t, fixture.RequestPayload.Model, fixture.BillingFields.Model)
			require.Equal(t, fixture.RequestPayload.Resolution, fixture.BillingFields.Resolution)
			require.Equal(t, int(*fixture.RequestPayload.Duration), fixture.BillingFields.Duration)

			if fixture.Scenario == "success" {
				require.Equal(t, fixture.BillingFields.Model, fixture.FinalResponsePayload.Model)
				require.Equal(t, fixture.BillingFields.Resolution, fixture.FinalResponsePayload.Resolution)
				require.Equal(t, fixture.BillingFields.Duration, fixture.FinalResponsePayload.Duration)
				require.Equal(t, *fixture.BillingFields.FPS, fixture.FinalResponsePayload.FramesPerSecond)
				require.Equal(t, *fixture.BillingFields.TotalTokens, fixture.FinalResponsePayload.Usage.TotalTokens)
				require.Equal(t, "placeholder", fixture.VideoArtifact.Kind)
				require.NotNil(t, fixture.VideoArtifact.URL)
				require.True(t, strings.HasPrefix(*fixture.VideoArtifact.URL, "https://example.invalid/"))
				return
			}

			require.Nil(t, fixture.BillingFields.FPS)
			require.Nil(t, fixture.BillingFields.TotalTokens)
			require.Nil(t, fixture.BillingFields.CompletionTokens)
		})
	}
}

func seedanceFixtureServer(t *testing.T, fixture seedanceFixture) *httptest.Server {
	t.Helper()
	submitResponse := mustMarshalBytes(t, fixture.SubmitResponsePayload)
	finalResponse := mustMarshalBytes(t, fixture.FinalResponsePayload)
	expectedPath := "/api/v3/contents/generations/tasks/" + fixture.SubmitResponsePayload.ID

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer fixture-test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/contents/generations/tasks":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.JSONEq(t, string(mustMarshalBytes(t, fixture.RequestPayload)), string(body))
			_, _ = w.Write(submitResponse)
		case r.Method == http.MethodGet && r.URL.Path == expectedPath:
			_, _ = w.Write(finalResponse)
		default:
			http.NotFound(w, r)
		}
	}))
}

func loadSeedanceFixture(t *testing.T, name string) seedanceFixture {
	t.Helper()
	var fixture seedanceFixture
	require.NoError(t, common.Unmarshal(readFixtureBytes(t, name), &fixture))
	return fixture
}

func readFixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name+".json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func mustMarshalBytes(t *testing.T, value any) []byte {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return data
}

func mustMarshalReader(t *testing.T, value any) io.Reader {
	t.Helper()
	return bytes.NewReader(mustMarshalBytes(t, value))
}

var _ = videobilling.ModeFormula
