package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOptionBundleContext(t *testing.T, method string, target string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func TestOptionBundleExportExcludesSensitiveKeys(t *testing.T) {
	setupGrowthControllerTestDB(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"SystemName":         "kittyvibe",
		"GitHubClientSecret": "super-secret",
		"TurnstileSiteKey":   "site-secret-too",
		"ModelRatio":         `{"gpt":1}`,
	}
	common.OptionMapRWMutex.Unlock()

	ctx, recorder := newOptionBundleContext(t, http.MethodGet, "/api/option/bundle/export", "")
	ExportOptionBundle(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Options map[string]string `json:"options"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "kittyvibe", response.Data.Options["SystemName"])
	require.NotContains(t, response.Data.Options, "GitHubClientSecret")
	require.NotContains(t, response.Data.Options, "TurnstileSiteKey")
}

func TestOptionBundleImportPreviewAndApplySkipsSensitiveKeys(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	require.NoError(t, model.UpdateOption("SystemName", "Before"))
	common.OptionMapRWMutex.Lock()
	common.OptionMap["GitHubClientSecret"] = "existing-secret"
	common.OptionMapRWMutex.Unlock()

	body := `{"bundle":{"version":1,"options":{"SystemName":"After","GitHubClientSecret":"leaked"}}}`
	ctx, recorder := newOptionBundleContext(t, http.MethodPost, "/api/option/bundle/import/preview", body)
	PreviewOptionBundleImport(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var preview struct {
		Success bool `json:"success"`
		Data    struct {
			Apply bool               `json:"apply"`
			Diffs []OptionBundleDiff `json:"diffs"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &preview))
	require.True(t, preview.Success)
	require.False(t, preview.Data.Apply)
	require.Len(t, preview.Data.Diffs, 2)
	require.Equal(t, "Before", common.OptionMap["SystemName"])

	ctx, recorder = newOptionBundleContext(t, http.MethodPost, "/api/option/bundle/import/apply", body)
	ApplyOptionBundleImport(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var applied struct {
		Success bool `json:"success"`
		Data    struct {
			Apply bool               `json:"apply"`
			Diffs []OptionBundleDiff `json:"diffs"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &applied))
	require.True(t, applied.Success)
	require.True(t, applied.Data.Apply)
	require.Equal(t, "After", common.OptionMap["SystemName"])
	require.Equal(t, "existing-secret", common.OptionMap["GitHubClientSecret"])

	var option model.Option
	require.NoError(t, db.First(&option, "key = ?", "SystemName").Error)
	require.Equal(t, "After", option.Value)
	require.Error(t, db.First(&option, "key = ?", "GitHubClientSecret").Error)
}
