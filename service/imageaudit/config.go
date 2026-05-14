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

// Region identifies which ARK Assets backend a record / audit call targets.
// "cn" → Volcano Ark (cn-beijing). "global" → BytePlus ModelArk (ap-southeast-1).
// Region is a string (not a typed enum) to keep the over-the-wire shape simple —
// the public POST /v1/image-audits accepts the same values verbatim.
//
// Empty / unknown values are normalized to RegionCN by NormalizeRegion so legacy
// callers continue to target the China backend.
const (
	RegionCN     = "cn"
	RegionGlobal = "global"
)

// NormalizeRegion maps incoming region hints to a canonical key. Treats
// channel-type-style aliases ("byteplus") as global. Empty falls back to CN
// so existing callers without a region parameter keep the prior behaviour.
func NormalizeRegion(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case RegionGlobal, "overseas", "byteplus":
		return RegionGlobal
	default:
		return RegionCN
	}
}

// Config bundles every env value the audit pipeline needs. Values are read
// once via Load(), TrimSpace'd to tolerate accidental "VAR=300   " padding,
// and stripped of inline "# ..." comments to tolerate dotenv files that
// keep comments next to the value.
type Config struct {
	// Volcano ARK Assets API (国内 / cn-beijing).
	ARKAk        string
	ARKSk        string
	ARKProject   string
	ARKGroupName string
	ARKGroupID   string

	// BytePlus ModelArk Assets API (海外 / ap-southeast-1). Independent
	// credential / project / group set — China and overseas Asset libraries
	// are separate stores; an asset audited in CN is not visible to BytePlus
	// and vice versa.
	ARKAkGlobal        string
	ARKSkGlobal        string
	ARKProjectGlobal   string
	ARKGroupNameGlobal string
	ARKGroupIDGlobal   string
	// ARKHostGlobal is the BytePlus Assets OpenAPI host. Configurable so an
	// operator can override if BytePlus moves the endpoint or routes to a
	// different region. Default matches the published BytePlus ap-southeast-1
	// pattern.
	ARKHostGlobal   string
	ARKRegionGlobal string

	// Polling settings — shared across regions.
	ARKPollInterval time.Duration
	ARKPollTimeout  time.Duration

	// Tencent COS (only required when callers send raw bytes; URL passthrough
	// skips this entirely). Shared between regions — COS is just storage to
	// give ARK a public URL; the upload target itself is region-agnostic.
	COSBucket     string
	COSRegion     string
	COSSecretID   string
	COSSecretKey  string
	COSPrefix     string
	COSPublicHost string
}

// Resolved is the per-region projection of Config — what arkClient needs to
// sign and dispatch one OpenAPI call. Decouples the call-site from "which
// region am I in" branching: every site builds a Resolved and proceeds.
type Resolved struct {
	Ak        string
	Sk        string
	Host      string
	Region    string // v4 signing region, e.g. "cn-beijing" / "ap-southeast-1"
	Service   string // "ark"
	Project   string
	GroupName string
	GroupID   string
}

// HasCreds is true when both AK and SK are present. Other fields can be
// empty (group falls back to defaults; project falls back to "default").
func (r Resolved) HasCreds() bool {
	return r.Ak != "" && r.Sk != ""
}

// Load reads the audit config from process env. It never errors — missing
// fields are reported by Validate*() at the moment they're actually needed.
func Load() Config {
	return Config{
		ARKAk:        cleanEnv("ARK_AK"),
		ARKSk:        cleanEnv("ARK_SK"),
		ARKProject:   defaultIfEmpty(cleanEnv("ARK_ASSETS_PROJECT"), "default"),
		ARKGroupName: defaultIfEmpty(cleanEnv("ARK_ASSETS_GROUP_NAME"), "aikanhub-audit"),
		ARKGroupID:   cleanEnv("ARK_ASSETS_GROUP_ID"),

		ARKAkGlobal:        cleanEnv("ARK_AK_GLOBAL"),
		ARKSkGlobal:        cleanEnv("ARK_SK_GLOBAL"),
		ARKProjectGlobal:   defaultIfEmpty(cleanEnv("ARK_ASSETS_PROJECT_GLOBAL"), "default"),
		ARKGroupNameGlobal: defaultIfEmpty(cleanEnv("ARK_ASSETS_GROUP_NAME_GLOBAL"), "aikanhub-audit"),
		ARKGroupIDGlobal:   cleanEnv("ARK_ASSETS_GROUP_ID_GLOBAL"),
		// Verified by smoke-testing CreateAsset/GetAsset against the BytePlus
		// 海外 control plane in ap-southeast-1. The `byteplusapi.com` apex
		// (not `bytepluses.com`) is BytePlus's TOB OpenAPI control plane,
		// where Volcengine v4 signing is accepted. The `bytepluses.com` host
		// only serves Bearer-auth inference traffic.
		ARKHostGlobal:      defaultIfEmpty(cleanEnv("ARK_HOST_GLOBAL"), "ark.ap-southeast-1.byteplusapi.com"),
		ARKRegionGlobal:    defaultIfEmpty(cleanEnv("ARK_REGION_GLOBAL"), "ap-southeast-1"),

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

// ResolveFor returns the credentials + host + project triple to use for the
// requested region. The bool is false when creds for that region aren't set.
func (c Config) ResolveFor(region string) (Resolved, bool) {
	switch NormalizeRegion(region) {
	case RegionGlobal:
		r := Resolved{
			Ak: c.ARKAkGlobal, Sk: c.ARKSkGlobal,
			Host: c.ARKHostGlobal, Region: c.ARKRegionGlobal, Service: "ark",
			Project: c.ARKProjectGlobal, GroupName: c.ARKGroupNameGlobal, GroupID: c.ARKGroupIDGlobal,
		}
		return r, r.HasCreds()
	default:
		r := Resolved{
			Ak: c.ARKAk, Sk: c.ARKSk,
			Host: "ark.cn-beijing.volcengineapi.com", Region: "cn-beijing", Service: "ark",
			Project: c.ARKProject, GroupName: c.ARKGroupName, GroupID: c.ARKGroupID,
		}
		return r, r.HasCreds()
	}
}

// RegionFromProject infers a record's region by matching its persisted Project
// name against the configured CN/Global project ids. Used by Refresh and Wait
// where the only signal of region origin is the stored record. Falls back to
// CN when the project doesn't match either side (e.g. legacy rows whose
// project was 'default' before separate config existed).
func (c Config) RegionFromProject(project string) string {
	p := strings.TrimSpace(project)
	if p != "" {
		if p == c.ARKProjectGlobal && c.ARKProjectGlobal != "" {
			return RegionGlobal
		}
		if p == c.ARKProject && c.ARKProject != "" {
			return RegionCN
		}
	}
	return RegionCN
}

// HasARK is true iff Volcano (CN) credentials are present. Kept for callers
// that explicitly target the CN region and don't yet thread region through.
func (c Config) HasARK() bool {
	return c.ARKAk != "" && c.ARKSk != ""
}

// HasARKGlobal is true iff BytePlus (overseas) credentials are present.
func (c Config) HasARKGlobal() bool {
	return c.ARKAkGlobal != "" && c.ARKSkGlobal != ""
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
