package billing_visibility_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestBillingVisibilityDefaults(t *testing.T) {
	resetBillingVisibilitySetting(t)

	got := GetBillingVisibilitySetting()
	require.Equal(t, "credits", got.DefaultMode)
	require.Equal(t, "credits", got.GroupModes["default"])
	require.Equal(t, "credits", got.GroupModes["trial"])
	require.Equal(t, "summary", got.GroupModes["beta"])
	require.Equal(t, "detailed", got.GroupModes["invited"])
	require.Equal(t, "detailed", got.GroupModes["b2b"])
	require.Equal(t, "detailed", got.GroupModes["enterprise"])
}

func TestBillingVisibilityLoadsFromConfig(t *testing.T) {
	resetBillingVisibilitySetting(t)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_visibility_setting.default_mode": "summary",
		"billing_visibility_setting.group_modes":  `{"default":"credits","partners":"detailed"}`,
	}))

	got := GetBillingVisibilitySetting()
	require.Equal(t, "summary", got.DefaultMode)
	require.Equal(t, "credits", got.GroupModes["default"])
	require.Equal(t, "detailed", got.GroupModes["partners"])
}

func resetBillingVisibilitySetting(t *testing.T) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_visibility_setting.default_mode": "credits",
		"billing_visibility_setting.group_modes":  `{"default":"credits","trial":"credits","beta":"summary","invited":"detailed","b2b":"detailed","enterprise":"detailed"}`,
	}))
}
