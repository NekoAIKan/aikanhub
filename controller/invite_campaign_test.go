package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/onboarding_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGrowthControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.UsingMySQL = false
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.Option{}, &model.InviteCampaign{}))
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	onboarding_setting.ResetForTest()
	t.Cleanup(func() {
		onboarding_setting.ResetForTest()
	})
	return db
}

func performRegister(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	Register(ctx)
	return recorder
}

func TestRegisterAppliesOnboardingPolicyQuotaGroupAndDefaultToken(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"onboarding_setting.new_user_quota":          "777",
		"onboarding_setting.default_group":           "trial",
		"onboarding_setting.default_token_enabled":   "1",
		"onboarding_setting.default_token_quota":     "333",
		"onboarding_setting.default_token_group":     "trial-token",
		"onboarding_setting.default_token_unlimited": "false",
	}))

	recorder := performRegister(t, `{"username":"trialuser","password":"password1"}`)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])

	var user model.User
	require.NoError(t, db.First(&user, "username = ?", "trialuser").Error)
	require.Equal(t, 777, user.Quota)
	require.Equal(t, "trial", user.Group)

	var token model.Token
	require.NoError(t, db.First(&token, "user_id = ?", user.Id).Error)
	require.Equal(t, 333, token.RemainQuota)
	require.False(t, token.UnlimitedQuota)
	require.Equal(t, "trial-token", token.Group)
}

func TestRegisterAppliesCampaignInvitePolicyAndIncrementsUsage(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.InviteCampaign{
		Code:       "LAUNCH",
		Name:       "Launch",
		Status:     model.InviteCampaignStatusEnabled,
		StartTime:  now - 10,
		EndTime:    now + 1000,
		UsageLimit: 1,
		Quota:      9900,
		Group:      "launch",
	}).Error)

	recorder := performRegister(t, `{"username":"launchuser","password":"password1","invite_code":"LAUNCH"}`)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])

	var user model.User
	require.NoError(t, db.First(&user, "username = ?", "launchuser").Error)
	require.Equal(t, 9900, user.Quota)
	require.Equal(t, "launch", user.Group)

	var campaign model.InviteCampaign
	require.NoError(t, db.First(&campaign, "code = ?", "LAUNCH").Error)
	require.Equal(t, 1, campaign.UsedCount)

	recorder = performRegister(t, `{"username":"launchuser2","password":"password1","invite_code":"LAUNCH"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, false, response["success"])
}

func TestGetSelfIncludesResolvedBillingVisibilityMode(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_visibility_setting.default_mode": "credits",
		"billing_visibility_setting.group_modes":  `{"default":"credits","trial":"credits","beta":"summary","invited":"detailed","b2b":"detailed","enterprise":"detailed"}`,
	}))
	require.NoError(t, db.Create(&model.User{
		Id:       90,
		Username: "b2b-user",
		Password: "password1",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "b2b",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Set("id", 90)
	ctx.Set("role", common.RoleCommonUser)

	GetSelf(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Group                 string `json:"group"`
			BillingVisibilityMode string `json:"billing_visibility_mode"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "b2b", response.Data.Group)
	require.Equal(t, "detailed", response.Data.BillingVisibilityMode)
}
