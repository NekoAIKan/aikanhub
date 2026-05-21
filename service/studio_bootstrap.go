package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	StudioCSRFHeader = "X-Studio-CSRF"
	studioCSRFMaxAge = 2 * time.Hour
)

type StudioBootstrapSession struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Group    string `json:"group"`
	Role     int    `json:"role"`
}

type StudioBootstrapAuth struct {
	Mode       string `json:"mode"`
	UserHeader string `json:"userHeader"`
	CSRFHeader string `json:"csrfHeader"`
	CSRFToken  string `json:"csrfToken"`
}

type StudioCredentialProfile struct {
	ID            string   `json:"id"`
	Label         string   `json:"label"`
	Source        string   `json:"source"`
	Scope         []string `json:"scope"`
	Status        string   `json:"status"`
	SecretExposed bool     `json:"secretExposed"`
}

type StudioBootstrapCapabilities struct {
	VideoGeneration           bool   `json:"videoGeneration"`
	TemporaryUploads          bool   `json:"temporaryUploads"`
	GeneratedVideoProxy       bool   `json:"generatedVideoProxy"`
	PricingPreview            bool   `json:"pricingPreview"`
	ModelCapabilitiesEndpoint string `json:"modelCapabilitiesEndpoint"`
	GenerationsEndpoint       string `json:"generationsEndpoint"`
	PricingPreviewEndpoint    string `json:"pricingPreviewEndpoint"`
	ContentProxyTemplate      string `json:"contentProxyTemplate"`
}

type StudioBootstrapData struct {
	Session            StudioBootstrapSession      `json:"session"`
	Auth               StudioBootstrapAuth         `json:"auth"`
	CredentialProfiles []StudioCredentialProfile   `json:"credentialProfiles"`
	Capabilities       StudioBootstrapCapabilities `json:"capabilities"`
}

func BuildStudioBootstrap(userID int, username string, group string, role int) (StudioBootstrapData, error) {
	token, err := GetOrCreateStudioToken(userID, group)
	if err != nil {
		return StudioBootstrapData{}, err
	}
	return newStudioBootstrapData(userID, username, group, role, token), nil
}

func newStudioBootstrapData(userID int, username string, group string, role int, token *model.Token) StudioBootstrapData {
	profileStatus := "not_configured"
	if token != nil && token.Status == common.TokenStatusEnabled && token.IsStudioManaged() {
		profileStatus = "ready"
	}
	return StudioBootstrapData{
		Session: StudioBootstrapSession{
			ID:       userID,
			Username: username,
			Group:    group,
			Role:     role,
		},
		Auth: StudioBootstrapAuth{
			Mode:       "kittyvibe-session",
			UserHeader: "New-Api-User",
			CSRFHeader: StudioCSRFHeader,
			CSRFToken:  GenerateStudioCSRFToken(userID, time.Now().Unix()),
		},
		CredentialProfiles: []StudioCredentialProfile{{
			ID:            "studio-default",
			Label:         StudioTokenName,
			Source:        "server-managed",
			Scope:         []string{"video:generate", "video:read"},
			Status:        profileStatus,
			SecretExposed: false,
		}},
		Capabilities: StudioBootstrapCapabilities{
			VideoGeneration:           true,
			TemporaryUploads:          true,
			GeneratedVideoProxy:       true,
			PricingPreview:            true,
			ModelCapabilitiesEndpoint: "/api/studio/video/models",
			GenerationsEndpoint:       "/api/studio/video/generations",
			PricingPreviewEndpoint:    "/api/studio/video/pricing/preview",
			ContentProxyTemplate:      "/v1/videos/{taskId}/content",
		},
	}
}

func GenerateStudioCSRFToken(userID int, issuedAt int64) string {
	payload := fmt.Sprintf("v1:%d:%d", userID, issuedAt)
	mac := hmac.New(sha256.New, []byte(common.SessionSecret))
	mac.Write([]byte(payload))
	signature := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func VerifyStudioCSRFToken(token string, userID int, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	payload := string(payloadBytes)
	mac := hmac.New(sha256.New, []byte(common.SessionSecret))
	mac.Write([]byte(payload))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return false
	}
	fields := strings.Split(payload, ":")
	if len(fields) != 3 || fields[0] != "v1" {
		return false
	}
	tokenUserID, err := strconv.Atoi(fields[1])
	if err != nil || tokenUserID != userID {
		return false
	}
	issuedAt, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return false
	}
	issuedTime := time.Unix(issuedAt, 0)
	return !issuedTime.After(now) && now.Sub(issuedTime) <= studioCSRFMaxAge
}
