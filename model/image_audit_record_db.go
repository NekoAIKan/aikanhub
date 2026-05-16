// image_audit_record_db.go — query/update helpers for ImageAuditRecord.
// Kept in a sibling file so the model definition stays a clean struct.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// recordIDPrefix matches the public-facing convention. Keep stable —
// users may persist these on their side.
const recordIDPrefix = "imgaudit_"

// NewImageAuditRecordID returns a fresh user-facing id like
// "imgaudit_3a4f...". 16 hex chars is plenty for the foreseeable
// volume and matches the visual weight of asset-... ids.
func NewImageAuditRecordID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the host is broken; surface up.
		// Fall back to time so we never panic from this helper.
		return fmt.Sprintf("%s%016x", recordIDPrefix, time.Now().UnixNano())
	}
	return recordIDPrefix + hex.EncodeToString(b)
}

// CreateImageAuditRecord inserts a new row, populating CreatedAt/UpdatedAt
// and RecordID if the caller didn't.
func CreateImageAuditRecord(r *ImageAuditRecord) error {
	if r.RecordID == "" {
		r.RecordID = NewImageAuditRecordID()
	}
	now := time.Now().Unix()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	return DB.Create(r).Error
}

// SaveImageAuditRecord persists changes to an existing row, bumping
// UpdatedAt. Use this from the poller and the lazy-refresh code path.
func SaveImageAuditRecord(r *ImageAuditRecord) error {
	r.Touch()
	return DB.Save(r).Error
}

// GetImageAuditRecordByRecordID looks up by the public id, scoped by
// userID so one user can't peek at another's records. Pass userID=0
// from internal call sites to bypass the scope (e.g. background poller).
func GetImageAuditRecordByRecordID(userID int, recordID string) (*ImageAuditRecord, error) {
	if recordID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var r ImageAuditRecord
	q := DB.Where("record_id = ?", recordID)
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// FindActiveImageAuditByHash returns the most recent reusable (active)
// record. Used when callers want to short-circuit only on confirmed
// passes — e.g. when they explicitly want to retry a previously-failed
// audit.
func FindActiveImageAuditByHash(userID int, sourceHash string) (*ImageAuditRecord, error) {
	if userID == 0 || sourceHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var r ImageAuditRecord
	err := DB.Where("user_id = ? AND source_hash = ? AND status = ?",
		userID, sourceHash, ImageAuditStatusActive).
		Order("id DESC").First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// FindTerminalImageAuditByHash is the dedup hot-path query. Returns
// the most recent terminal (active OR failed) row for this source.
// Failed rows are cached because moderation decisions are deterministic
// for the same content — re-running just spends ARK quota and surprises
// the user with an inconsistent verdict.
//
// Active rows take priority over failed when both exist for the same
// (user, hash): we ORDER BY id DESC then prefer the active one in
// memory, since GORM gives us a single result.
//
// Kept as a tested helper; the per-region cache path uses
// FindTerminalImageAuditByHashAndProject which additionally scopes by the
// audit project (so CN and overseas don't share cache entries).
func FindTerminalImageAuditByHash(userID int, sourceHash string) (*ImageAuditRecord, error) {
	if userID == 0 || sourceHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	// Prefer active. If none, fall back to the latest failed.
	if r, err := FindActiveImageAuditByHash(userID, sourceHash); err == nil {
		return r, nil
	}
	var r ImageAuditRecord
	err := DB.Where("user_id = ? AND source_hash = ? AND status = ?",
		userID, sourceHash, ImageAuditStatusFailed).
		Order("id DESC").First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// FindTerminalImageAuditByHashAndProject is the region-aware cache lookup.
// Filters by project so a CN record (project=shemao) is never returned for
// a global lookup (project=tianwenyue-6) — China and overseas ARK Asset
// stores are entirely independent; an asset audited in one is invalid in
// the other. Empty project is treated as "any project" for backward
// compatibility with rows written before region-aware caching landed.
//
// Active rows preferred over failed, same as FindTerminalImageAuditByHash.
func FindTerminalImageAuditByHashAndProject(userID int, sourceHash, project string) (*ImageAuditRecord, error) {
	if userID == 0 || sourceHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if project == "" {
		return FindTerminalImageAuditByHash(userID, sourceHash)
	}
	// Prefer active. If none, fall back to the latest failed.
	var r ImageAuditRecord
	activeErr := DB.Where("user_id = ? AND source_hash = ? AND project = ? AND status = ?",
		userID, sourceHash, project, ImageAuditStatusActive).
		Order("id DESC").First(&r).Error
	if activeErr == nil {
		return &r, nil
	}
	err := DB.Where("user_id = ? AND source_hash = ? AND project = ? AND status = ?",
		userID, sourceHash, project, ImageAuditStatusFailed).
		Order("id DESC").First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// FindImageAuditByAssetID resolves an ARK "asset-..." id back to its
// gateway record. Used when the doubao adaptor receives a raw asset
// reference and wants to confirm we've audited it (so re-runs don't
// hit ARK at all).
func FindImageAuditByAssetID(assetID string) (*ImageAuditRecord, error) {
	if assetID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var r ImageAuditRecord
	err := DB.Where("asset_id = ?", assetID).Order("id DESC").First(&r).Error
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// FindImageAuditByAssetIDAndRegion resolves a previously-audited asset for
// the target ARK region. userID > 0 scopes the lookup to the caller.
func FindImageAuditByAssetIDAndRegion(userID int, assetID, region string) (*ImageAuditRecord, error) {
	if assetID == "" || region == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var r ImageAuditRecord
	q := DB.Where("asset_id = ? AND region = ?", assetID, region)
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Order("id DESC").First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// FindLegacyImageAuditByAssetIDAndProject resolves legacy rows that predate
// the explicit Region column. userID > 0 scopes the lookup to the caller.
func FindLegacyImageAuditByAssetIDAndProject(userID int, assetID, project string) (*ImageAuditRecord, error) {
	if assetID == "" || project == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var r ImageAuditRecord
	q := DB.Where("asset_id = ? AND project = ? AND (region = ? OR region IS NULL)", assetID, project, "")
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Order("id DESC").First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

// ListImageAuditRecords returns the user's records newest-first, paged.
// page is 1-based; pageSize is clamped to [1, 100] to keep responses
// bounded. Returns (rows, total, error).
func ListImageAuditRecords(userID, page, pageSize int) ([]ImageAuditRecord, int64, error) {
	if userID <= 0 {
		return nil, 0, errors.New("userID required")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var total int64
	if err := DB.Model(&ImageAuditRecord{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []ImageAuditRecord
	err := DB.Where("user_id = ?", userID).
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error
	return rows, total, err
}

// FindStaleProcessingImageAudits returns processing rows whose UpdatedAt
// is older than `cutoff`. Startup sweeper uses this to find orphaned
// pollers (process restart killed the goroutine that owned the row).
func FindStaleProcessingImageAudits(cutoff time.Time) ([]ImageAuditRecord, error) {
	var rows []ImageAuditRecord
	err := DB.Where("status = ? AND updated_at < ?",
		ImageAuditStatusProcessing, cutoff.Unix()).
		Find(&rows).Error
	return rows, err
}
