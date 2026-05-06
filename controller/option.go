package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func isVisiblePublicKeyOption(key string) bool {
	switch key {
	case "WaffoPancakeWebhookPublicKey", "WaffoPancakeWebhookTestKey":
		return true
	default:
		return false
	}
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		value := common.Interface2String(v)
		isSensitiveKey := strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key")
		if isSensitiveKey && !isVisiblePublicKeyOption(k) {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
	return
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func normalizeOptionUpdateValue(value any) string {
	switch v := value.(type) {
	case bool:
		return common.Interface2String(v)
	case float64:
		return common.Interface2String(v)
	case int:
		return common.Interface2String(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func validateFloatRatioMapOption(value string, label string) error {
	var parsed map[string]float64
	if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
		return fmt.Errorf("%s设置失败: %w", label, err)
	}
	for name, ratio := range parsed {
		if ratio < 0 {
			return fmt.Errorf("%s设置失败: %s 的倍率不能小于 0", label, name)
		}
	}
	return nil
}

func validateOptionUpdate(key string, value string) error {
	switch key {
	case "GitHubOAuthEnabled":
		if value == "true" && common.GitHubClientId == "" {
			return fmt.Errorf("无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！")
		}
	case "discord.enabled":
		if value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			return fmt.Errorf("无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！")
		}
	case "oidc.enabled":
		if value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			return fmt.Errorf("无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！")
		}
	case "LinuxDOOAuthEnabled":
		if value == "true" && common.LinuxDOClientId == "" {
			return fmt.Errorf("无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！")
		}
	case "EmailDomainRestrictionEnabled":
		if value == "true" && len(common.EmailDomainWhitelist) == 0 {
			return fmt.Errorf("无法启用邮箱域名限制，请先填入限制的邮箱域名！")
		}
	case "WeChatAuthEnabled":
		if value == "true" && common.WeChatServerAddress == "" {
			return fmt.Errorf("无法启用微信登录，请先填入微信登录相关配置信息！")
		}
	case "TurnstileCheckEnabled":
		if value == "true" && common.TurnstileSiteKey == "" {
			return fmt.Errorf("无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！")
		}
	case "TelegramOAuthEnabled":
		if value == "true" && common.TelegramBotToken == "" {
			return fmt.Errorf("无法启用 Telegram OAuth，请先填入 Telegram Bot Token！")
		}
	case "theme.frontend":
		if value != "default" {
			return fmt.Errorf("无效的主题值，仅支持 default")
		}
	case "GroupRatio":
		if err := ratio_setting.CheckGroupRatio(value); err != nil {
			return err
		}
	case "ImageRatio":
		if err := validateFloatRatioMapOption(value, "图片倍率"); err != nil {
			return err
		}
	case "AudioRatio":
		if err := validateFloatRatioMapOption(value, "音频倍率"); err != nil {
			return err
		}
	case "AudioCompletionRatio":
		if err := validateFloatRatioMapOption(value, "音频补全倍率"); err != nil {
			return err
		}
	case "CreateCacheRatio":
		if err := validateFloatRatioMapOption(value, "缓存创建倍率"); err != nil {
			return err
		}
	case "ModelRequestRateLimitGroup":
		if err := setting.CheckModelRequestRateLimitGroup(value); err != nil {
			return err
		}
	case "AutomaticDisableStatusCodes":
		if _, err := operation_setting.ParseHTTPStatusCodeRanges(value); err != nil {
			return err
		}
	case "AutomaticRetryStatusCodes":
		if _, err := operation_setting.ParseHTTPStatusCodeRanges(value); err != nil {
			return err
		}
	case "console_setting.api_info":
		if err := console_setting.ValidateConsoleSettings(value, "ApiInfo"); err != nil {
			return err
		}
	case "console_setting.announcements":
		if err := console_setting.ValidateConsoleSettings(value, "Announcements"); err != nil {
			return err
		}
	case "console_setting.faq":
		if err := console_setting.ValidateConsoleSettings(value, "FAQ"); err != nil {
			return err
		}
	case "console_setting.uptime_kuma_groups":
		if err := console_setting.ValidateConsoleSettings(value, "UptimeKumaGroups"); err != nil {
			return err
		}
	}
	return nil
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	optionValue := normalizeOptionUpdateValue(option.Value)
	if err := validateOptionUpdate(option.Key, optionValue); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = model.UpdateOption(option.Key, optionValue)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}
