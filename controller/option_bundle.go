package controller

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type OptionBundle struct {
	SchemaVersion int               `json:"schema_version,omitempty"`
	Version       int               `json:"version,omitempty"`
	ExportedAt    int64             `json:"exported_at,omitempty"`
	App           string            `json:"app,omitempty"`
	Options       map[string]string `json:"options"`
	Redacted      []string          `json:"redacted,omitempty"`
}

type OptionBundleImportRequest struct {
	Bundle OptionBundle `json:"bundle"`
}

type OptionBundleDiff struct {
	Key           string `json:"key"`
	CurrentValue  string `json:"current_value"`
	IncomingValue string `json:"incoming_value"`
	Action        string `json:"action"`
	Blocked       bool   `json:"blocked"`
	Message       string `json:"message,omitempty"`
}

type OptionBundleSummary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Blocked   int `json:"blocked"`
	Rejected  int `json:"rejected"`
}

func isOptionBundleBlockedKey(key string) bool {
	if key == "" || key == "SchemaMigrationHash" {
		return true
	}
	lowerKey := strings.ToLower(key)
	if strings.HasSuffix(key, "Token") ||
		strings.HasSuffix(key, "Secret") ||
		strings.HasSuffix(key, "Key") ||
		strings.HasSuffix(lowerKey, "secret") ||
		strings.HasSuffix(lowerKey, "api_key") {
		return !isVisiblePublicKeyOption(key)
	}
	return false
}

func sanitizedOptionBundleOptions() (map[string]string, []string) {
	options := make(map[string]string)
	redacted := make([]string, 0)
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	for key, value := range common.OptionMap {
		if isOptionBundleBlockedKey(key) {
			redacted = append(redacted, key)
			continue
		}
		options[key] = common.Interface2String(value)
	}
	sort.Strings(redacted)
	return options, redacted
}

func ExportOptionBundle(c *gin.Context) {
	options, redacted := sanitizedOptionBundleOptions()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": OptionBundle{
			SchemaVersion: 2,
			ExportedAt:    common.GetTimestamp(),
			App:           "aikanhub",
			Options:       options,
			Redacted:      redacted,
		},
	})
}

func PreviewOptionBundleImport(c *gin.Context) {
	handleOptionBundleImport(c, false)
}

func ApplyOptionBundleImport(c *gin.Context) {
	handleOptionBundleImport(c, true)
}

func handleOptionBundleImport(c *gin.Context, apply bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiErrorMsg(c, "无效的配置包")
		return
	}
	bundle, err := decodeOptionBundleImportBody(body)
	if err != nil {
		common.ApiErrorMsg(c, "无效的配置包")
		return
	}
	if err := validateOptionBundle(bundle); err != nil {
		common.ApiError(c, err)
		return
	}
	diffs := buildOptionBundleDiffs(bundle)
	summary := summarizeOptionBundleDiffs(diffs)
	if apply && summary.Rejected > 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "配置包包含无效配置项，请先修正后再应用",
			"data": gin.H{
				"apply":   apply,
				"diffs":   diffs,
				"summary": summary,
			},
		})
		return
	}
	appliedKeys := make([]string, 0)
	if apply {
		for _, diff := range diffs {
			if diff.Blocked || diff.Action == "unchanged" || diff.Action == "rejected" {
				continue
			}
			if err := model.UpdateOption(diff.Key, diff.IncomingValue); err != nil {
				common.ApiError(c, err)
				return
			}
			appliedKeys = append(appliedKeys, diff.Key)
		}
		if len(appliedKeys) > 0 {
			sort.Strings(appliedKeys)
			adminInfo := map[string]interface{}{
				"username": c.GetString("username"),
				"keys":     appliedKeys,
			}
			model.RecordLogWithAdminInfo(
				c.GetInt("id"),
				model.LogTypeManage,
				fmt.Sprintf("应用配置包，更新键: %s", strings.Join(appliedKeys, ", ")),
				adminInfo,
			)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"apply":   apply,
			"diffs":   diffs,
			"summary": summary,
		},
	})
}

func decodeOptionBundleImportBody(body []byte) (OptionBundle, error) {
	var bundle OptionBundle
	if err := common.Unmarshal(body, &bundle); err != nil {
		return OptionBundle{}, err
	}
	if bundle.Options != nil {
		return bundle, nil
	}

	var req OptionBundleImportRequest
	if err := common.Unmarshal(body, &req); err != nil {
		return OptionBundle{}, err
	}
	return req.Bundle, nil
}

func validateOptionBundle(bundle OptionBundle) error {
	if bundle.SchemaVersion != 0 && bundle.SchemaVersion != 2 {
		return fmt.Errorf("不支持的配置包版本: %d", bundle.SchemaVersion)
	}
	if bundle.SchemaVersion == 0 && bundle.Version != 0 && bundle.Version != 1 {
		return fmt.Errorf("不支持的配置包版本: %d", bundle.Version)
	}
	if bundle.Options == nil {
		return fmt.Errorf("无效的配置包")
	}
	return nil
}

func summarizeOptionBundleDiffs(diffs []OptionBundleDiff) OptionBundleSummary {
	var summary OptionBundleSummary
	for _, diff := range diffs {
		switch diff.Action {
		case "create":
			summary.Create++
		case "update":
			summary.Update++
		case "unchanged":
			summary.Unchanged++
		case "blocked":
			summary.Blocked++
		case "rejected":
			summary.Rejected++
		}
	}
	return summary
}

func buildOptionBundleDiffs(bundle OptionBundle) []OptionBundleDiff {
	keys := make([]string, 0, len(bundle.Options))
	for key := range bundle.Options {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	diffs := make([]OptionBundleDiff, 0, len(keys))
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	for _, key := range keys {
		incoming := bundle.Options[key]
		current, exists := common.OptionMap[key]
		diff := OptionBundleDiff{
			Key:           key,
			CurrentValue:  current,
			IncomingValue: incoming,
			Blocked:       isOptionBundleBlockedKey(key),
		}
		validationErr := validateOptionUpdate(key, incoming)
		switch {
		case diff.Blocked:
			diff.Action = "blocked"
		case validationErr != nil:
			diff.Action = "rejected"
			diff.Message = validationErr.Error()
		case !exists:
			diff.Action = "create"
		case current == incoming:
			diff.Action = "unchanged"
		default:
			diff.Action = "update"
		}
		diffs = append(diffs, diff)
	}
	return diffs
}
