package onboarding_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestOnboardingPolicyLoadsAdminConfigAndPreservesExplicitZeroQuota(t *testing.T) {
	originalDefaultToken := constant.GenerateDefaultToken
	constant.GenerateDefaultToken = true
	t.Cleanup(func() {
		constant.GenerateDefaultToken = originalDefaultToken
		ResetForTest()
	})
	ResetForTest()

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"onboarding_setting.new_user_quota":          "0",
		"onboarding_setting.default_group":           "beta",
		"onboarding_setting.default_token_enabled":   "0",
		"onboarding_setting.default_token_quota":     "12345",
		"onboarding_setting.default_token_group":     "launch",
		"onboarding_setting.default_token_unlimited": "false",
	}))

	policy := GetPolicy()
	require.Equal(t, 0, policy.NewUserQuota)
	require.Equal(t, "beta", policy.DefaultGroup)
	require.False(t, policy.GenerateDefaultToken)
	require.Equal(t, 12345, policy.DefaultTokenQuota)
	require.Equal(t, "launch", policy.DefaultTokenGroup)
	require.False(t, policy.DefaultTokenUnlimited)
}

func TestOnboardingPolicyFallsBackToLegacyDefaultTokenEnv(t *testing.T) {
	originalDefaultToken := constant.GenerateDefaultToken
	constant.GenerateDefaultToken = true
	t.Cleanup(func() {
		constant.GenerateDefaultToken = originalDefaultToken
		ResetForTest()
	})
	ResetForTest()

	policy := GetPolicy()
	require.True(t, policy.GenerateDefaultToken)
	require.Equal(t, "default", policy.DefaultGroup)
	require.Equal(t, 500000, policy.DefaultTokenQuota)
	require.True(t, policy.DefaultTokenUnlimited)
}
