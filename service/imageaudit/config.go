// Package imageaudit gates image-bearing video tasks behind Volcano ARK's
// asset library audit. The flow is:
//
//	user image (URL or base64)
//	    ↓
//	(if base64) upload to Tencent COS → public URL
//	    ↓
//	ARK Assets API: CreateAsset(URL) → poll GetAsset until Active
//	    ↓
//	asset://<id>  ← gateway substitutes this in the upstream payload
//
// Triggered per-request via metadata.audit_image=true. Default off so existing
// callers see no change.
package imageaudit

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config bundles every env value the audit pipeline needs. Values are read
// once via Load(), TrimSpace'd to tolerate accidental "VAR=300   " padding,
// and stripped of inline "# ..." comments to tolerate dotenv files that
// keep comments next to the value.
type Config struct {
	// Volcano ARK Assets API.
	ARKAk            string
	ARKSk            string
	ARKProject       string
	ARKGroupName     string
	ARKGroupID       string
	ARKPollInterval  time.Duration
	ARKPollTimeout   time.Duration

	// Tencent COS (only required when callers send raw bytes; URL passthrough
	// skips this entirely).
	COSBucket     string
	COSRegion     string
	COSSecretID   string
	COSSecretKey  string
	COSPrefix     string
	COSPublicHost string
}

// Load reads the audit config from process env. It never errors — missing
// fields are reported by Validate*() at the moment they're actually needed.
func Load() Config {
	return Config{
		ARKAk:           cleanEnv("ARK_AK"),
		ARKSk:           cleanEnv("ARK_SK"),
		ARKProject:      defaultIfEmpty(cleanEnv("ARK_ASSETS_PROJECT"), "default"),
		ARKGroupName:    defaultIfEmpty(cleanEnv("ARK_ASSETS_GROUP_NAME"), "aikanhub-audit"),
		ARKGroupID:      cleanEnv("ARK_ASSETS_GROUP_ID"),
		ARKPollInterval: time.Duration(parseIntDefault(cleanEnv("ARK_ASSETS_POLL_INTERVAL"), 3)) * time.Second,
		ARKPollTimeout:  time.Duration(parseIntDefault(cleanEnv("ARK_ASSETS_POLL_TIMEOUT"), 300)) * time.Second,

		COSBucket:     cleanEnv("COS_BUCKET"),
		COSRegion:     defaultIfEmpty(cleanEnv("COS_REGION"), "ap-shanghai"),
		COSSecretID:   cleanEnv("COS_SECRET_ID"),
		COSSecretKey:  cleanEnv("COS_SECRET_KEY"),
		COSPrefix:     defaultIfEmpty(cleanEnv("COS_OBJECT_PREFIX"), "aikanhub-audit/"),
		COSPublicHost: cleanEnv("COS_PUBLIC_HOST"),
	}
}

// HasARK is true iff Volcano credentials are present. CreateAsset / GetAsset
// require both AK and SK; we don't validate format.
func (c Config) HasARK() bool {
	return c.ARKAk != "" && c.ARKSk != ""
}

// HasCOS is true iff Tencent COS credentials and bucket are present.
func (c Config) HasCOS() bool {
	return c.COSBucket != "" && c.COSSecretID != "" && c.COSSecretKey != ""
}

// COSEndpoint returns the canonical "{bucket}.cos.{region}.myqcloud.com" host
// used for both signing and the upload URL itself. PublicHost is only used
// when handing the URL back to ARK; signing always goes against the cos.*
// endpoint.
func (c Config) COSEndpoint() string {
	return c.COSBucket + ".cos." + c.COSRegion + ".myqcloud.com"
}

// PublicURL builds the externally-reachable URL given an object key. Falls
// back to the canonical endpoint when COS_PUBLIC_HOST is empty.
func (c Config) PublicURL(objectKey string) string {
	host := strings.TrimRight(c.COSPublicHost, "/")
	if host == "" {
		host = "https://" + c.COSEndpoint()
	}
	if !strings.HasPrefix(objectKey, "/") {
		objectKey = "/" + objectKey
	}
	return host + objectKey
}

// cleanEnv reads VAR, trims whitespace, and strips a trailing inline
// "# ..." comment when one slipped in. Standard dotenv parsers tolerate this
// inconsistently; trimming it here means env files like
//
//	ARK_ASSETS_POLL_TIMEOUT=300   # 秒
//
// load as 300 instead of "300   # 秒".
func cleanEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		return ""
	}
	// Don't strip # inside quoted values — but our env values never have #
	// legitimately, so a simple split is fine.
	if i := strings.Index(v, "#"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

func defaultIfEmpty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func parseIntDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
