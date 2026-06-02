package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAttachTaskRequestSnapshotRecordsPromptAndPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)

	c.Set(relaycommon.TaskRequestContextKey, relaycommon.TaskSubmitReq{
		Model:      "doubao-seedance-2-0-fast-260128",
		Prompt:     "debug marker: orange robot walks left",
		Duration:   5,
		Resolution: "720p",
	})
	relaycommon.SetTaskUpstreamRequest(c, []byte(`{"model":"doubao-seedance-2-0-fast-260128","content":[{"type":"text","text":"debug marker: orange robot walks left"}]}`))

	task := &model.Task{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         17,
			ChannelType:       constant.ChannelTypeVolcEngine,
			UpstreamModelName: "doubao-seedance-2-0-fast-260128",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionTextGenerate},
	}
	info.OriginModelName = "doubao-seedance-2-0-fast-260128"

	platform := constant.TaskPlatform("54")
	attachTaskRequestSnapshot(c, task, info, platform)

	require.Equal(t, "debug marker: orange robot walks left", task.Properties.Input)
	require.NotNil(t, task.PrivateData.RequestSnapshot)
	require.Equal(t, "debug marker: orange robot walks left", task.PrivateData.RequestSnapshot.Prompt)
	require.Equal(t, "doubao-seedance-2-0-fast-260128", task.PrivateData.RequestSnapshot.Model)
	require.Equal(t, constant.TaskActionTextGenerate, task.PrivateData.RequestSnapshot.Action)
	require.Equal(t, string(platform), task.PrivateData.RequestSnapshot.Platform)
	require.Equal(t, 17, task.PrivateData.RequestSnapshot.ChannelID)
	require.Equal(t, constant.ChannelTypeVolcEngine, task.PrivateData.RequestSnapshot.ChannelType)
	require.Contains(t, string(task.PrivateData.RequestSnapshot.NormalizedRequest), "orange robot")
	require.Contains(t, string(task.PrivateData.RequestSnapshot.UpstreamRequest), "orange robot")
}
