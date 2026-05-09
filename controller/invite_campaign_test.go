package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/onboarding_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type testOAuthSession map[interface{}]interface{}

func (s testOAuthSession) ID() string { return "test-session" }
func (s testOAuthSession) Get(key interface{}) interface{} {
	return s[key]
}
func (s testOAuthSession) Set(key interface{}, val interface{}) {
	s[key] = val
}
func (s testOAuthSession) Delete(key interface{}) {
	delete(s, key)
}
func (s testOAuthSession) Clear() {
	for key := range s {
		delete(s, key)
	}
}
func (s testOAuthSession) AddFlash(value interface{}, vars ...string) {}
func (s testOAuthSession) Flashes(vars ...string) []interface{}       { return nil }
func (s testOAuthSession) Options(options sessions.Options)           {}
func (s testOAuthSession) Save() error                                { return nil }

type testOAuthProvider struct{}

func (p *testOAuthProvider) GetName() string { return "Test OAuth" }
func (p *testOAuthProvider) IsEnabled() bool { return true }
func (p *testOAuthProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{AccessToken: "test-token"}, nil
}
func (p *testOAuthProvider) GetUserInfo(ctx context.Context, token *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return &oauth.OAuthUser{ProviderUserID: "oauth-user"}, nil
}
func (p *testOAuthProvider) IsUserIDTaken(providerUserID string) bool { return false }
func (p *testOAuthProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	return nil
}
func (p *testOAuthProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.OidcId = providerUserID
}
func (p *testOAuthProvider) GetProviderPrefix() string { return "oauth_" }

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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.Option{}, &model.InviteCampaign{}, &model.MoneyWallet{}, &model.MoneyWalletTransaction{}))
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	onboarding_setting.ResetForTest()
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "legacy",
		"billing_setting.settlement_currency": "USD",
	}))
	t.Cleanup(func() {
		onboarding_setting.ResetForTest()
		_ = config.GlobalConfig.LoadFromDB(map[string]string{
			"billing_setting.money_billing_mode":  "legacy",
			"billing_setting.settlement_currency": "USD",
		})
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
		"onboarding_setting.new_user_quota":             "777",
		"onboarding_setting.default_group":              "trial",
		"onboarding_setting.default_token_enabled":      "1",
		"onboarding_setting.default_token_quota":        "333",
		"onboarding_setting.default_token_group":        "trial-token",
		"onboarding_setting.default_token_unlimited":    "false",
		"onboarding_setting.default_token_expire_days":  "7",
		"onboarding_setting.default_token_model_limits": "gpt-4o,doubao-seedance-2-0-fast-260128",
	}))
	before := common.GetTimestamp()

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
	require.GreaterOrEqual(t, token.ExpiredTime, before+7*86400)
	require.LessOrEqual(t, token.ExpiredTime, common.GetTimestamp()+7*86400)
	require.True(t, token.ModelLimitsEnabled)
	require.Equal(t, "gpt-4o,doubao-seedance-2-0-fast-260128", token.ModelLimits)

	var logs []model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeSystem).Find(&logs).Error)
	contents := make([]string, 0, len(logs))
	for _, log := range logs {
		contents = append(contents, log.Content)
	}
	joined := strings.Join(contents, "\n")
	require.Contains(t, joined, "初始令牌策略")
	require.Contains(t, joined, "已创建")
	require.Contains(t, joined, "333")
	require.Contains(t, joined, "7 天")
	require.Contains(t, joined, "trial-token")
	require.Contains(t, joined, "gpt-4o,doubao-seedance-2-0-fast-260128")
}

func TestRegisterLogsDisabledDefaultTokenPolicy(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"onboarding_setting.default_token_enabled": "0",
	}))

	recorder := performRegister(t, `{"username":"notokenuser","password":"password1"}`)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])

	var user model.User
	require.NoError(t, db.First(&user, "username = ?", "notokenuser").Error)
	var tokenCount int64
	require.NoError(t, db.Model(&model.Token{}).Where("user_id = ?", user.Id).Count(&tokenCount).Error)
	require.Zero(t, tokenCount)

	var logs []model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeSystem).Find(&logs).Error)
	contents := make([]string, 0, len(logs))
	for _, log := range logs {
		contents = append(contents, log.Content)
	}
	joined := strings.Join(contents, "\n")
	require.Contains(t, joined, "初始令牌策略")
	require.Contains(t, joined, "未创建")
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

func TestRegisterRequiresActiveCampaignInviteWhenPolicyEnabled(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"onboarding_setting.require_invite_campaign_code": "true",
	}))
	require.NoError(t, db.Create(&model.User{
		Username: "inviter",
		Password: "password1",
		AffCode:  "AFF123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.InviteCampaign{
		Code:       "BETA",
		Name:       "Beta",
		Status:     model.InviteCampaignStatusEnabled,
		StartTime:  now - 10,
		EndTime:    now + 1000,
		UsageLimit: 2,
		Quota:      1200,
		Group:      "beta",
	}).Error)
	require.NoError(t, db.Create(&model.InviteCampaign{
		Code:   "OFF",
		Name:   "Disabled",
		Status: model.InviteCampaignStatusDisabled,
		Quota:  1200,
	}).Error)

	require.NoError(t, db.Create(&model.InviteCampaign{
		Code:       "OLD",
		Name:       "Expired",
		Status:     model.InviteCampaignStatusEnabled,
		StartTime:  now - 1000,
		EndTime:    now - 1,
		UsageLimit: 1,
		Quota:      1200,
	}).Error)
	require.NoError(t, db.Create(&model.InviteCampaign{
		Code:       "FULL",
		Name:       "Full",
		Status:     model.InviteCampaignStatusEnabled,
		UsageLimit: 1,
		UsedCount:  1,
		Quota:      1200,
	}).Error)

	for name, tc := range map[string]struct {
		body    string
		message string
	}{
		"missing":   {body: `{"username":"missinguser","password":"password1"}`, message: "需要有效的邀请码"},
		"affiliate": {body: `{"username":"affiliateuser","password":"password1","aff_code":"AFF123"}`, message: "无效的邀请码"},
		"disabled":  {body: `{"username":"disableduser","password":"password1","invite_code":"OFF"}`, message: "邀请码已停用"},
		"expired":   {body: `{"username":"expireduser","password":"password1","invite_code":"OLD"}`, message: "邀请码已过期"},
		"exhausted": {body: `{"username":"fulluser","password":"password1","invite_code":"FULL"}`, message: "邀请码已用完"},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := performRegister(t, tc.body)
			require.Equal(t, http.StatusOK, recorder.Code)
			var response map[string]any
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, false, response["success"])
			require.Contains(t, response["message"], tc.message)
		})
	}

	recorder := performRegister(t, `{"username":"betauser","password":"password1","invite_code":"BETA"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])

	var user model.User
	require.NoError(t, db.First(&user, "username = ?", "betauser").Error)
	require.Equal(t, 1200, user.Quota)
	require.Equal(t, "beta", user.Group)
}

func TestOAuthSignupRequiresActiveCampaignInviteWhenPolicyEnabled(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"onboarding_setting.require_invite_campaign_code": "true",
	}))
	require.NoError(t, db.Create(&model.User{
		Username: "inviter",
		Password: "password1",
		AffCode:  "AFF123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&model.InviteCampaign{
		Code:       "OAUTHBETA",
		Name:       "OAuth Beta",
		Status:     model.InviteCampaignStatusEnabled,
		StartTime:  now - 10,
		EndTime:    now + 1000,
		UsageLimit: 2,
		Quota:      2200,
		Group:      "oauth-beta",
	}).Error)
	require.NoError(t, db.Create(&model.InviteCampaign{
		Code:   "OAUTHOFF",
		Name:   "OAuth Disabled",
		Status: model.InviteCampaignStatusDisabled,
		Quota:  2200,
	}).Error)

	provider := &testOAuthProvider{}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	_, err := findOrCreateOAuthUser(ctx, provider, &oauth.OAuthUser{
		ProviderUserID: "oauth-missing",
		Username:       "oauthmissing",
	}, testOAuthSession{})
	require.ErrorIs(t, err, model.ErrInviteCampaignRequired)

	affiliateSession := testOAuthSession{}
	affiliateSession.Set("aff", "AFF123")
	_, err = findOrCreateOAuthUser(ctx, provider, &oauth.OAuthUser{
		ProviderUserID: "oauth-affiliate",
		Username:       "oauthaffiliate",
	}, affiliateSession)
	require.ErrorIs(t, err, model.ErrInviteCampaignNotFound)

	disabledSession := testOAuthSession{}
	disabledSession.Set("aff", "OAUTHOFF")
	_, err = findOrCreateOAuthUser(ctx, provider, &oauth.OAuthUser{
		ProviderUserID: "oauth-disabled",
		Username:       "oauthdisabled",
	}, disabledSession)
	require.ErrorIs(t, err, model.ErrInviteCampaignDisabled)

	activeSession := testOAuthSession{}
	activeSession.Set("aff", "OAUTHBETA")
	user, err := findOrCreateOAuthUser(ctx, provider, &oauth.OAuthUser{
		ProviderUserID: "oauth-active",
		Username:       "oauthactive",
		DisplayName:    "OAuth Active",
	}, activeSession)
	require.NoError(t, err)
	require.Equal(t, 2200, user.Quota)
	require.Equal(t, "oauth-beta", user.Group)

	var stored model.User
	require.NoError(t, db.First(&stored, "username = ?", "oauthactive").Error)
	require.Equal(t, "oauth-active", stored.OidcId)
	require.Equal(t, 2200, stored.Quota)
	require.Equal(t, "oauth-beta", stored.Group)
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

func TestGetSelfIncludesMoneyWalletInMoneyMode(t *testing.T) {
	db := setupGrowthControllerTestDB(t)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.money_billing_mode":  "money",
		"billing_setting.settlement_currency": "USD",
	}))
	require.NoError(t, db.Create(&model.User{
		Id:       91,
		Username: "money-user",
		Password: "password1",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    12345,
	}).Error)
	require.NoError(t, model.CreditWallet(91, "USD", 2_500_000, "get-self-money-wallet", model.MoneyWalletTransactionTopup, "test"))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Set("id", 91)
	ctx.Set("role", common.RoleCommonUser)

	GetSelf(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Quota                    int    `json:"quota"`
			MoneyBillingMode         string `json:"money_billing_mode"`
			MoneyBalanceAmountMicros int64  `json:"money_balance_amount_micros"`
			MoneyCurrency            string `json:"money_currency"`
			MoneyWallet              struct {
				Currency            string `json:"currency"`
				AvailableMicros     int64  `json:"available_micros"`
				FrozenMicros        int64  `json:"frozen_micros"`
				LifetimeTopupMicros int64  `json:"lifetime_topup_micros"`
			} `json:"money_wallet"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 12345, response.Data.Quota)
	require.Equal(t, "money", response.Data.MoneyBillingMode)
	require.Equal(t, "USD", response.Data.MoneyCurrency)
	require.Equal(t, int64(2_500_000), response.Data.MoneyBalanceAmountMicros)
	require.Equal(t, int64(2_500_000), response.Data.MoneyWallet.AvailableMicros)
	require.Equal(t, int64(2_500_000), response.Data.MoneyWallet.LifetimeTopupMicros)
}
