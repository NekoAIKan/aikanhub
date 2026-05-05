package onboarding_setting

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
)

const legacyDefaultTokenEnabled = -1

type OnboardingSetting struct {
	NewUserQuota          int    `json:"new_user_quota"`
	DefaultGroup          string `json:"default_group"`
	DefaultTokenEnabled   int    `json:"default_token_enabled"`
	DefaultTokenQuota     int    `json:"default_token_quota"`
	DefaultTokenGroup     string `json:"default_token_group"`
	DefaultTokenUnlimited bool   `json:"default_token_unlimited"`
}

type Policy struct {
	NewUserQuota          int
	DefaultGroup          string
	GenerateDefaultToken  bool
	DefaultTokenQuota     int
	DefaultTokenGroup     string
	DefaultTokenUnlimited bool
}

var onboardingSetting OnboardingSetting

func init() {
	ResetForTest()
	config.GlobalConfig.Register("onboarding_setting", &onboardingSetting)
}

func ResetForTest() {
	onboardingSetting = OnboardingSetting{
		NewUserQuota:          -1,
		DefaultGroup:          "default",
		DefaultTokenEnabled:   legacyDefaultTokenEnabled,
		DefaultTokenQuota:     500000,
		DefaultTokenUnlimited: true,
	}
}

func GetPolicy() Policy {
	newUserQuota := onboardingSetting.NewUserQuota
	if newUserQuota < 0 {
		newUserQuota = common.QuotaForNewUser
	}

	defaultGroup := onboardingSetting.DefaultGroup
	if defaultGroup == "" {
		defaultGroup = "default"
	}

	generateDefaultToken := constant.GenerateDefaultToken
	if onboardingSetting.DefaultTokenEnabled != legacyDefaultTokenEnabled {
		generateDefaultToken = onboardingSetting.DefaultTokenEnabled > 0
	}

	defaultTokenQuota := onboardingSetting.DefaultTokenQuota
	if defaultTokenQuota < 0 {
		defaultTokenQuota = 0
	}

	defaultTokenGroup := onboardingSetting.DefaultTokenGroup
	if defaultTokenGroup == "" && setting.DefaultUseAutoGroup {
		defaultTokenGroup = "auto"
	}

	return Policy{
		NewUserQuota:          newUserQuota,
		DefaultGroup:          defaultGroup,
		GenerateDefaultToken:  generateDefaultToken,
		DefaultTokenQuota:     defaultTokenQuota,
		DefaultTokenGroup:     defaultTokenGroup,
		DefaultTokenUnlimited: onboardingSetting.DefaultTokenUnlimited,
	}
}
