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
	require.False(t, policy.RequireInviteCampaignCode)
	require.Equal(t, 0, policy.DefaultTokenExpireDays)
	require.Equal(t, "", policy.DefaultTokenModelLimits)
}

func TestOnboardingPolicyLoadsInviteGateTokenExpiryAndModelLimits(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"onboarding_setting.require_invite_campaign_code": "true",
		"onboarding_setting.default_token_expire_days":    "14",
		"onboarding_setting.default_token_model_limits":   "gpt-4o,doubao-seedance-2-0-fast-260128",
	}))

	policy := GetPolicy()
	require.True(t, policy.RequireInviteCampaignCode)
	require.Equal(t, 14, policy.DefaultTokenExpireDays)
	require.Equal(t, "gpt-4o,doubao-seedance-2-0-fast-260128", policy.DefaultTokenModelLimits)
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
