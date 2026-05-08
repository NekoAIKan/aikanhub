package imageaudit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// EnsureAudited drives the full audit pipeline for a single image source and
// returns the asset:// URI to substitute back into the upstream payload.
//
// src is whatever the user gave us:
//
//	http(s)://...                  → CreateAsset(src) → poll → asset://<id>
//	data:image/...;base64,<b64>    → COS upload → CreateAsset(public URL) → poll → asset://<id>
//	<bare base64 string>           → same as data: URI, MIME inferred
//	asset://<id>                   → returned as-is (already-audited inputs are fine)
//
// On audit failure the error is *AuditError. On infra failures (creds missing,
// COS upload failed, ARK 5xx, etc.) the error is a plain wrapped error.
func EnsureAudited(ctx context.Context, src string) (string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", errors.New("imageaudit: empty image source")
	}
	if strings.HasPrefix(src, "asset://") {
		// Already an audited reference — the caller pre-uploaded.
		return src, nil
	}

	cfg := Load()
	if !cfg.HasARK() {
		return "", errors.New("imageaudit: ARK_AK / ARK_SK not configured")
	}

	// Step 1 — make sure we have a public URL to give CreateAsset.
	publicURL, err := materializeURL(ctx, cfg, src)
	if err != nil {
		return "", fmt.Errorf("imageaudit: prepare URL: %w", err)
	}

	client := newARKClient(cfg)
	groupId, err := client.ensureAssetGroup(ctx)
	if err != nil {
		return "", fmt.Errorf("imageaudit: ensureAssetGroup: %w", err)
	}

	// Step 2 — submit + poll.
	assetID, err := client.createAsset(ctx, groupId, publicURL)
	if err != nil {
		return "", fmt.Errorf("imageaudit: createAsset: %w", err)
	}
	if _, err := client.pollAsset(ctx, assetID); err != nil {
		// AuditError is preserved verbatim so callers can map to a stable code.
		return "", err
	}
	return "asset://" + assetID, nil
}

// IsAuditFailure returns true when the error is the audit terminating in
// Failed (vs. transient infra errors). Used by the relay layer to map to a
// distinct user-facing error code.
func IsAuditFailure(err error) bool {
	var ae *AuditError
	return errors.As(err, &ae)
}

// materializeURL ensures the audit pipeline has a public HTTPS URL to feed
// CreateAsset. Plain http(s) URLs pass through; data URIs and bare base64
// strings get uploaded to COS first.
func materializeURL(ctx context.Context, cfg Config, src string) (string, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return src, nil
	}
	// Anything else has to be uploadable bytes. Decode + upload.
	if !cfg.HasCOS() {
		return "", errors.New("audit_image=true with non-URL source requires COS_BUCKET / COS_SECRET_ID / COS_SECRET_KEY")
	}
	raw, contentType, err := decodeDataURI(src)
	if err != nil {
		return "", fmt.Errorf("decode data URI: %w", err)
	}
	if len(raw) == 0 {
		return "", errors.New("decoded image is empty")
	}
	objectKey := buildObjectKey(cfg.COSPrefix, contentType)
	publicURL, err := uploadToCOS(ctx, cfg, objectKey, raw, contentType)
	if err != nil {
		return "", fmt.Errorf("cos upload: %w", err)
	}
	return publicURL, nil
}

// buildObjectKey produces a unique, short, predictable key like
// "<prefix>/2026/05/07/<8-hex>.jpg" so collisions are impossible and traces
// are sortable by date.
func buildObjectKey(prefix, contentType string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		prefix = "aikanhub-audit"
	}
	now := time.Now().UTC()
	day := now.Format("2006/01/02")
	rb := make([]byte, 6)
	_, _ = rand.Read(rb)
	suffix := hex.EncodeToString(rb)
	return fmt.Sprintf("%s/%s/%d-%s%s", prefix, day, now.UnixNano(), suffix, extOf(contentType))
}

// ---------------------------------------------------------------------------
// Single-flight cache: when a video task carries multiple images that all need
// auditing, the calling adapter loops over them serially. That's fine, but
// callers that batch identical URLs (rare, but happens with templated
// requests) shouldn't re-submit the same one. The cache below is process-
// local and bounded — it's only optimization, never correctness.
// ---------------------------------------------------------------------------

type cachedResult struct {
	asset    string
	expireAt time.Time
}

var (
	cacheMu sync.Mutex
	cache   = make(map[string]cachedResult)
)

const cacheTTL = 30 * time.Minute

// EnsureAuditedCached is EnsureAudited with a same-process best-effort cache
// keyed by the source string. Use this from the hot path; bare EnsureAudited
// is fine for tests.
func EnsureAuditedCached(ctx context.Context, src string) (string, error) {
	if cached, ok := cacheGet(src); ok {
		return cached, nil
	}
	out, err := EnsureAudited(ctx, src)
	if err != nil {
		return "", err
	}
	cachePut(src, out)
	return out, nil
}

func cacheGet(key string) (string, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	c, ok := cache[key]
	if !ok {
		return "", false
	}
	if time.Now().After(c.expireAt) {
		delete(cache, key)
		return "", false
	}
	return c.asset, true
}

func cachePut(key, asset string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if len(cache) > 1000 {
		// Crude eviction — drop everything. Audit decisions can re-derive on
		// next request; we only care about avoiding unbounded growth.
		cache = make(map[string]cachedResult)
	}
	cache[key] = cachedResult{asset: asset, expireAt: time.Now().Add(cacheTTL)}
}
