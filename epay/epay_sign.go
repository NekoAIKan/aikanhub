package main

import (
	"crypto/hmac"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
)

func SignEPayParams(params map[string]string, key string) string {
	filtered := make(map[string]string, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		filtered[k] = v
	}

	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+filtered[k])
	}

	sum := md5.Sum([]byte(strings.Join(parts, "&") + key))
	return fmt.Sprintf("%x", sum)
}

func AddEPaySignature(params map[string]string, key string) map[string]string {
	signed := make(map[string]string, len(params)+2)
	for k, v := range params {
		signed[k] = v
	}
	signed["sign"] = SignEPayParams(signed, key)
	signed["sign_type"] = "MD5"
	return signed
}

func VerifyEPayParams(params map[string]string, key string) bool {
	sign := params["sign"]
	if sign == "" {
		return false
	}
	expected := SignEPayParams(params, key)
	return hmac.Equal([]byte(sign), []byte(expected))
}
