package imageaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// EnsureAudited (legacy synchronous helper) and the new Submit/Wait
// pair both live here. Submit/Wait back the public /v1/image-audits
// endpoint; EnsureAudited keeps the per-request `audit_image=true` path
// working without forcing every caller to refactor.
//
// Design: no background goroutines. Submit persists the audit and
// returns immediately. Terminal-state discovery happens on demand in
// the GET path (and the Wait loop) via RefreshFromUpstream. Two
// independent concerns kept independent:
//
//	Submit              → write a row, hand back its id (fast)
//	GET / Wait          → read DB; if non-terminal, ask ARK once,
//	                      persist if it settled, return the latest row
//
// Storage shape:
//
//	src (URL or data URI)
//	    │
//	    ▼  hashSource()
//	(user_id, sha256) ──── DB lookup (terminal row?) ──── yes ──→ return cached
//	    │ no
//	    ▼
//	materializeURL  → COS upload if base64
//	    │
//	    ▼
//	ARK CreateAsset → assetID
//	    │
//	    ▼
//	model.CreateImageAuditRecord(status=processing)
//
// Cache hits never call ARK. Failed cache hits are NOT auto-retried —
// moderation rejections are deterministic and retrying just spends
// ARK quota for the same verdict.

// hashSource is the dedup key. Trimmed first so trailing whitespace
// from copy/paste still hits the cache.
func hashSource(src string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(src)))
	return hex.EncodeToString(sum[:])
}

func sourceKindOf(src string) model.ImageAuditSource {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return model.ImageAuditSourceURL
	}
	return model.ImageAuditSourceBase64
}

// Submit creates a fresh audit record (or returns a cached terminal
// one). No background work is started; callers drive the lifecycle by
// re-reading via GET (which refreshes upstream on demand) or by using
// Wait() to block. The returned record's status may be:
//
//   - active     — cache hit on a previously-passed audit. Reusable.
//   - failed     — cache hit on a previously-failed audit. Caller
//                  should NOT auto-retry; moderation decisions are
//                  deterministic per content, retrying just spends
//                  ARK quota.
//   - processing — fresh audit; ARK is working on it. The first GET
//                  (or Wait) will refresh from upstream.
//
// userID > 0 scopes the dedup cache to that user. userID=0 disables
// caching — internal calls without a user context fall straight to
// fresh audit.
//
// asset://<id> input is a degenerate case used by the legacy
// audit_image=true flow: a passthrough request for an asset the caller
// claims is already audited. We try to find OUR record for it (scoped
// to userID so we don't leak cross-tenant); if we don't have one, we
// synthesize a non-persisted record with status=active. Public callers
// (POST /v1/image-audits) should reject asset:// at the controller —
// the synthetic record's id is not GET-able.
func Submit(ctx context.Context, src string, userID, tokenID int) (*model.ImageAuditRecord, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, errors.New("imageaudit: empty source")
	}

	if strings.HasPrefix(src, "asset://") {
		return submitAssetPassthrough(src, userID, tokenID)
	}

	sh := hashSource(src)

	// Cache lookup over BOTH active and failed terminal rows. A failed
	// hit returns the cached failure verbatim; the caller decides how
	// to surface it. Skipping the cache for failed rows would mean
	// every retry of a moderation-rejected image spends another ARK
	// CreateAsset call for an identical (deterministic) outcome.
	//
	// IMPORTANT: cache check runs BEFORE the HasARK() guard so that
	// pre-audited rows remain queryable even if ARK credentials get
	// unset (e.g. ops rotates keys). Otherwise a config gap would
	// invalidate every cached audit, surprising callers with infra
	// errors on hot-path lookups.
	if userID > 0 {
		if cached, err := model.FindTerminalImageAuditByHash(userID, sh); err == nil {
			return cached, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			// Cache lookup is best-effort. Log and fall through to a
			// fresh audit so a transient DB hiccup doesn't block users.
			common.SysLog(fmt.Sprintf("imageaudit: cache lookup failed: %v", err))
		}
	}

	cfg := Load()
	if !cfg.HasARK() {
		return nil, errors.New("imageaudit: ARK_AK / ARK_SK not configured")
	}

	publicURL, err := materializeURL(ctx, cfg, src)
	if err != nil {
		return nil, fmt.Errorf("imageaudit: prepare URL: %w", err)
	}

	client := newARKClient(cfg)
	groupID, err := client.ensureAssetGroup(ctx)
	if err != nil {
		return nil, fmt.Errorf("imageaudit: ensureAssetGroup: %w", err)
	}
	assetID, err := client.createAsset(ctx, groupID, publicURL)
	if err != nil {
		return nil, fmt.Errorf("imageaudit: createAsset: %w", err)
	}

	rec := &model.ImageAuditRecord{
		UserID:     userID,
		TokenID:    tokenID,
		SourceHash: sh,
		SourceKind: sourceKindOf(src),
		PublicURL:  publicURL,
		Project:    cfg.ARKProject,
		GroupID:    groupID,
		AssetID:    assetID,
		AssetURI:   "asset://" + assetID,
		Status:     model.ImageAuditStatusProcessing,
	}
	if err := model.CreateImageAuditRecord(rec); err != nil {
		// Row creation failed but ARK already accepted the submission.
		// Log loud — the asset is now orphaned in ARK from our DB's
		// perspective. Caller gets an error and can resubmit.
		common.SysLog(fmt.Sprintf("imageaudit: persist record failed (assetID=%s orphaned): %v",
			assetID, err))
		return nil, fmt.Errorf("imageaudit: persist record: %w", err)
	}

	return rec, nil
}

// submitAssetPassthrough handles src="asset://<id>" inputs. Used by
// EnsureAudited (legacy audit_image=true) for callers who already have
// an audited asset id and want passthrough. The public POST endpoint
// rejects this case at the controller layer.
//
// We look up our own audit record for this asset, scoped to userID
// when provided, so user A can't probe whether user B has audited a
// given asset. If we don't have a record, we synthesize an in-memory
// active record — the caller asserts the asset is good; if it isn't,
// ARK will reject the downstream video task.
func submitAssetPassthrough(src string, userID, tokenID int) (*model.ImageAuditRecord, error) {
	assetID := strings.TrimPrefix(src, "asset://")
	if assetID == "" {
		return nil, errors.New("imageaudit: empty asset id")
	}
	if r, err := model.FindImageAuditByAssetID(assetID); err == nil {
		// Only return the existing record when it belongs to this
		// caller. Otherwise treat as unknown and synthesize, so we
		// don't leak userIDs / timestamps across tenants.
		if userID == 0 || r.UserID == userID {
			return r, nil
		}
	}
	now := time.Now().Unix()
	return &model.ImageAuditRecord{
		RecordID:  "external_" + assetID,
		UserID:    userID,
		TokenID:   tokenID,
		AssetID:   assetID,
		AssetURI:  src,
		Status:    model.ImageAuditStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Wait blocks until the record reaches a terminal state, ctx expires,
// or the audit poll timeout fires. Each iteration calls
// RefreshFromUpstream, which means every replica can drive any record
// — no shared in-process state. Backs the `?wait=true` convenience for
// scripts; production callers should poll GET themselves.
func Wait(ctx context.Context, recordID string) (*model.ImageAuditRecord, error) {
	cfg := Load()
	interval := cfg.ARKPollInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	deadline := time.Now().Add(cfg.ARKPollTimeout)
	for {
		r, err := model.GetImageAuditRecordByRecordID(0, recordID)
		if err != nil {
			return nil, err
		}
		if r.Status.IsTerminal() {
			return r, nil
		}
		r, _ = RefreshFromUpstream(ctx, r)
		if r.Status.IsTerminal() {
			return r, nil
		}
		if time.Now().After(deadline) {
			return r, fmt.Errorf("imageaudit: wait timed out (record=%s)", recordID)
		}
		select {
		case <-ctx.Done():
			return r, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// EnsureAudited is the legacy synchronous wrapper. Submit+Wait, return
// the asset:// URI on success. Audit failure → *AuditError so callers
// can map to a stable image_audit_failed error code.
//
// userID/tokenID are best-effort — pass 0 from callers that don't have
// the context yet (they'll just miss the dedup cache).
func EnsureAudited(ctx context.Context, src string, userID, tokenID int) (string, error) {
	rec, err := Submit(ctx, src, userID, tokenID)
	if err != nil {
		return "", err
	}
	if !rec.Status.IsTerminal() {
		rec, err = Wait(ctx, rec.RecordID)
		if err != nil {
			return "", err
		}
	}
	if rec.Status == model.ImageAuditStatusFailed {
		return "", &AuditError{AssetId: rec.AssetID, Status: "Failed", Reason: rec.Reason}
	}
	if rec.AssetURI == "" {
		return "", fmt.Errorf("imageaudit: record %s active but URI empty", rec.RecordID)
	}
	return rec.AssetURI, nil
}

// IsAuditFailure reports whether the error is an audit moderation
// rejection (vs. an infrastructure error). Callers map this to a stable
// image_audit_failed error code in the public response.
func IsAuditFailure(err error) bool {
	var ae *AuditError
	return errors.As(err, &ae)
}
