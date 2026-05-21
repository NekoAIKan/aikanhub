package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStudioSessionShapeRemainsStable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set("id", 101)
	ctx.Set("username", "creator")
	ctx.Set("group", "default")
	ctx.Set("role", 1)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/studio/session", nil)

	StudioSession(ctx)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ID       int    `json:"id"`
			Username string `json:"username"`
			Group    string `json:"group"`
			Role     int    `json:"role"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Equal(t, 101, resp.Data.ID)
	require.Equal(t, "creator", resp.Data.Username)
	require.Equal(t, "default", resp.Data.Group)
	require.Equal(t, 1, resp.Data.Role)
}
