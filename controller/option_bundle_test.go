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
			SchemaVersion int               `json:"schema_version"`
			App           string            `json:"app"`
			ExportedAt    int64             `json:"exported_at"`
			Options       map[string]string `json:"options"`
			Redacted      []string          `json:"redacted"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 2, response.Data.SchemaVersion)
	require.Equal(t, "aikanhub", response.Data.App)
	require.NotZero(t, response.Data.ExportedAt)
	require.Equal(t, "kittyvibe", response.Data.Options["SystemName"])
	require.NotContains(t, response.Data.Options, "GitHubClientSecret")
	require.NotContains(t, response.Data.Options, "TurnstileSiteKey")
	require.ElementsMatch(t, []string{"GitHubClientSecret", "TurnstileSiteKey"}, response.Data.Redacted)
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

func TestOptionBundleImportAcceptsRawSchemaV2Bundle(t *testing.T) {
	setupGrowthControllerTestDB(t)
	model.InitOptionMap()
	require.NoError(t, model.UpdateOption("SystemName", "Before"))

	body := `{"schema_version":2,"app":"aikanhub","options":{"SystemName":"RawAfter"}}`
	ctx, recorder := newOptionBundleContext(t, http.MethodPost, "/api/option/bundle/import/apply", body)
	ApplyOptionBundleImport(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Apply bool `json:"apply"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.True(t, response.Data.Apply)
	require.Equal(t, "RawAfter", common.OptionMap["SystemName"])
}

func TestOptionBundleImportAcceptsRawLegacyBundle(t *testing.T) {
	setupGrowthControllerTestDB(t)
	model.InitOptionMap()
	require.NoError(t, model.UpdateOption("SystemName", "Before"))

	body := `{"version":1,"options":{"SystemName":"LegacyAfter"}}`
	ctx, recorder := newOptionBundleContext(t, http.MethodPost, "/api/option/bundle/import/apply", body)
	ApplyOptionBundleImport(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Apply bool `json:"apply"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.True(t, response.Data.Apply)
	require.Equal(t, "LegacyAfter", common.OptionMap["SystemName"])
}

func TestOptionBundleImportPreviewRejectsInvalidOptionsWithoutMutation(t *testing.T) {
	setupGrowthControllerTestDB(t)
	model.InitOptionMap()
	require.NoError(t, model.UpdateOption("GroupRatio", `{"default":1}`))
	require.NoError(t, model.UpdateOption("SystemName", "Before"))

	body := `{"bundle":{"schema_version":2,"app":"aikanhub","options":{"GroupRatio":"not-json"}}}`
	ctx, recorder := newOptionBundleContext(t, http.MethodPost, "/api/option/bundle/import/preview", body)
	PreviewOptionBundleImport(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var preview struct {
		Success bool `json:"success"`
		Data    struct {
			Summary struct {
				Rejected int `json:"rejected"`
			} `json:"summary"`
			Diffs []OptionBundleDiff `json:"diffs"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &preview))
	require.True(t, preview.Success)
	require.Equal(t, 1, preview.Data.Summary.Rejected)
	require.Len(t, preview.Data.Diffs, 1)
	require.Equal(t, "rejected", preview.Data.Diffs[0].Action)
	require.NotEmpty(t, preview.Data.Diffs[0].Message)
	require.Equal(t, `{"default":1}`, common.OptionMap["GroupRatio"])
}

func TestOptionBundleApplyRejectsInvalidOptionsWithoutMutation(t *testing.T) {
	setupGrowthControllerTestDB(t)
	model.InitOptionMap()
	require.NoError(t, model.UpdateOption("GroupRatio", `{"default":1}`))

	body := `{"bundle":{"schema_version":2,"app":"aikanhub","options":{"GroupRatio":"not-json","SystemName":"After"}}}`
	ctx, recorder := newOptionBundleContext(t, http.MethodPost, "/api/option/bundle/import/apply", body)
	ApplyOptionBundleImport(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Summary struct {
				Rejected int `json:"rejected"`
			} `json:"summary"`
			Diffs []OptionBundleDiff `json:"diffs"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Equal(t, 1, response.Data.Summary.Rejected)
	require.Len(t, response.Data.Diffs, 2)
	require.Equal(t, `{"default":1}`, common.OptionMap["GroupRatio"])
	require.Equal(t, "Before", common.OptionMap["SystemName"])
}

func TestOptionBundleApplyWritesAuditLog(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	model.InitOptionMap()
	require.NoError(t, model.UpdateOption("SystemName", "Before"))

	body := `{"bundle":{"schema_version":2,"app":"aikanhub","options":{"SystemName":"After"}}}`
	ctx, recorder := newOptionBundleContext(t, http.MethodPost, "/api/option/bundle/import/apply", body)
	ctx.Set("id", 42)
	ctx.Set("username", "root")
	ApplyOptionBundleImport(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var log model.Log
	require.NoError(t, db.Last(&log).Error)
	require.Equal(t, 42, log.UserId)
	require.Equal(t, model.LogTypeManage, log.Type)
	require.Contains(t, log.Content, "配置包")
	require.Contains(t, log.Content, "SystemName")
}
