package profit_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestProfitSettingDefaultsPreserveLegacyVideoUnitPrice(t *testing.T) {
	resetProfitSetting(t)

	got := GetProfitSetting()
	require.Zero(t, got.DefaultMarkupPercent)
	require.Zero(t, got.UpstreamCostPerMillionTokens)
	require.True(t, got.ApplyToDefaultVideoProfiles)
}

func TestProfitSettingLoadsFromConfig(t *testing.T) {
	resetProfitSetting(t)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.default_markup_percent":           "30",
		"profit_setting.upstream_cost_per_million_tokens": "1.25",
		"profit_setting.apply_to_default_video_profiles":  "false",
	}))

	got := GetProfitSetting()
	require.Equal(t, 30.0, got.DefaultMarkupPercent)
	require.Equal(t, 1.25, got.UpstreamCostPerMillionTokens)
	require.False(t, got.ApplyToDefaultVideoProfiles)
}

func resetProfitSetting(t *testing.T) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.default_markup_percent":           "0",
		"profit_setting.upstream_cost_per_million_tokens": "0",
		"profit_setting.apply_to_default_video_profiles":  "true",
	}))
}
