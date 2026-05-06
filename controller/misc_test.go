package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetStatusIncludesCreditDisplayDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			QuotaPerUnit  float64 `json:"quota_per_unit"`
			CreditDisplay struct {
				Enabled        bool   `json:"enabled"`
				Label          string `json:"label"`
				QuotaPerCredit int    `json:"quota_per_credit"`
				Precision      int    `json:"precision"`
			} `json:"credit_display"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, common.QuotaPerUnit, response.Data.QuotaPerUnit)
	require.True(t, response.Data.CreditDisplay.Enabled)
	require.Equal(t, "Credits", response.Data.CreditDisplay.Label)
	require.Equal(t, 5000, response.Data.CreditDisplay.QuotaPerCredit)
	require.Equal(t, 2, response.Data.CreditDisplay.Precision)
}
