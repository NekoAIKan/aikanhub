package credit_display_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestCreditDisplayDefaults(t *testing.T) {
	resetCreditDisplaySetting(t)

	got := GetCreditDisplaySetting()
	require.True(t, got.Enabled)
	require.Equal(t, "Credits", got.Label)
	require.Equal(t, 5000, got.QuotaPerCredit)
	require.Equal(t, 2, got.Precision)
}

func TestCreditDisplayLoadsFromConfig(t *testing.T) {
	resetCreditDisplaySetting(t)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"credit_display_setting.enabled":          "false",
		"credit_display_setting.label":            "Points",
		"credit_display_setting.quota_per_credit": "2500",
		"credit_display_setting.precision":        "3",
	}))

	got := GetCreditDisplaySetting()
	require.False(t, got.Enabled)
	require.Equal(t, "Points", got.Label)
	require.Equal(t, 2500, got.QuotaPerCredit)
	require.Equal(t, 3, got.Precision)
}

func resetCreditDisplaySetting(t *testing.T) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"credit_display_setting.enabled":          "true",
		"credit_display_setting.label":            "Credits",
		"credit_display_setting.quota_per_credit": "5000",
		"credit_display_setting.precision":        "2",
	}))
}
