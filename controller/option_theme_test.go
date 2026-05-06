package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionRejectsClassicTheme(t *testing.T) {
	setupGrowthControllerTestDB(t)
	model.InitOptionMap()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"theme.frontend": "default",
	}))

	ctx, recorder := newOptionBundleContext(t, http.MethodPut, "/api/option/", `{"key":"theme.frontend","value":"classic"}`)
	UpdateOption(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Contains(t, response.Message, "default")
	require.NotContains(t, response.Message, "classic")
	require.Equal(t, "default", system_setting.GetThemeSettings().Frontend)
	require.Equal(t, "default", common.OptionMap["theme.frontend"])
}

func TestUpdateOptionAcceptsDefaultTheme(t *testing.T) {
	setupGrowthControllerTestDB(t)
	model.InitOptionMap()

	ctx, recorder := newOptionBundleContext(t, http.MethodPut, "/api/option/", `{"key":"theme.frontend","value":"default"}`)
	UpdateOption(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "default", system_setting.GetThemeSettings().Frontend)
	require.Equal(t, "default", common.OptionMap["theme.frontend"])
}
