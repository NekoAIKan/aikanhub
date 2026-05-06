package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestBillingVisibilityModeResolvesRoleAndGroup(t *testing.T) {
	resetBillingVisibilityForServiceTest(t, "credits")

	require.Equal(t, BillingVisibilityCredits, GetBillingVisibilityMode(common.RoleCommonUser, "default"))
	require.Equal(t, BillingVisibilityCredits, GetBillingVisibilityMode(common.RoleCommonUser, "trial"))
	require.Equal(t, BillingVisibilitySummary, GetBillingVisibilityMode(common.RoleCommonUser, "beta"))
	require.Equal(t, BillingVisibilityDetailed, GetBillingVisibilityMode(common.RoleCommonUser, "invited"))
	require.Equal(t, BillingVisibilityDetailed, GetBillingVisibilityMode(common.RoleCommonUser, "b2b"))
	require.Equal(t, BillingVisibilityDetailed, GetBillingVisibilityMode(common.RoleCommonUser, "enterprise"))
	require.Equal(t, BillingVisibilityInternal, GetBillingVisibilityMode(common.RoleAdminUser, "default"))
	require.Equal(t, BillingVisibilityInternal, GetBillingVisibilityMode(common.RoleRootUser, "default"))
}

func TestBillingVisibilityUnknownGroupFallsBackToDefaultMode(t *testing.T) {
	resetBillingVisibilityForServiceTest(t, "summary")

	require.Equal(t, BillingVisibilitySummary, GetBillingVisibilityMode(common.RoleCommonUser, "unknown-group"))
}

func resetBillingVisibilityForServiceTest(t *testing.T, defaultMode string) {
	t.Helper()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_visibility_setting.default_mode": defaultMode,
		"billing_visibility_setting.group_modes":  `{"default":"credits","trial":"credits","beta":"summary","invited":"detailed","b2b":"detailed","enterprise":"detailed"}`,
	}))
}
