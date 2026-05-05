package controller

import (
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type OptionBundle struct {
	Version int               `json:"version"`
	Options map[string]string `json:"options"`
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

func sanitizedOptionBundleOptions() map[string]string {
	options := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	for key, value := range common.OptionMap {
		if isOptionBundleBlockedKey(key) {
			continue
		}
		options[key] = common.Interface2String(value)
	}
	return options
}

func ExportOptionBundle(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": OptionBundle{
			Version: 1,
			Options: sanitizedOptionBundleOptions(),
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
	var req OptionBundleImportRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的配置包")
		return
	}
	diffs := buildOptionBundleDiffs(req.Bundle)
	if apply {
		for _, diff := range diffs {
			if diff.Blocked || diff.Action == "unchanged" {
				continue
			}
			if err := model.UpdateOption(diff.Key, diff.IncomingValue); err != nil {
				common.ApiError(c, err)
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"apply": apply,
			"diffs": diffs,
		},
	})
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
		switch {
		case diff.Blocked:
			diff.Action = "blocked"
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
