// Package model — image_audit_record.go persists the result of an image
// audit job submitted to Volcano ARK's Asset library.
//
// Why this exists: pre-existing `metadata.audit_image=true` flow used a
// process-local map to dedupe identical inputs. That fell over on multi-
// replica deploys and across restarts. This table replaces that map and
// also backs the public /v1/image-audits endpoint that lets advanced
// callers explicitly pre-audit images and reuse the resulting asset id
// without paying audit latency on every video task.
package model

import (
	"time"
)

// ImageAuditStatus is the persisted audit lifecycle state. We mirror
// Volcano's "Processing/Active/Failed" but lowercase for JSON friendliness
// — clients who consume our API never see ARK's raw casing.
type ImageAuditStatus string

const (
	ImageAuditStatusProcessing ImageAuditStatus = "processing"
	ImageAuditStatusActive     ImageAuditStatus = "active"
	ImageAuditStatusFailed     ImageAuditStatus = "failed"
)

// IsTerminal reports whether the audit will not change state again.
// Used by the lookup cache (only Active is reusable) and the sweeper
// (Processing past timeout → infra orphan, mark Failed).
func (s ImageAuditStatus) IsTerminal() bool {
	return s == ImageAuditStatusActive || s == ImageAuditStatusFailed
}

// ImageAuditSource describes how the user supplied the image. Lets us
// avoid storing massive base64 strings — we only keep the hash and the
// fully-resolved public URL we handed to ARK.
type ImageAuditSource string

const (
	ImageAuditSourceURL    ImageAuditSource = "url"
	ImageAuditSourceBase64 ImageAuditSource = "base64"
)

// ImageAuditRecord is one audit job, owned by a single user.
//
// SourceHash is the dedup key: SHA-256 of the trimmed source string the
// user submitted. We never compare base64 bytes directly (too expensive)
// — the hash is enough to short-circuit repeat submissions of the same
// data URI.
//
// AssetID is Volcano's "asset-YYYYMMDDHHMMSS-xxxxx" id. AssetURI is
// "asset://<AssetID>" pre-built so callers can paste it straight into
// video generation requests.
//
// Composite index (user_id, source_hash) backs the dedup lookup. Both
// columns share the same `index:idx_user_source,...` tag.
type ImageAuditRecord struct {
	ID         int64            `json:"-" gorm:"primary_key;AUTO_INCREMENT"`
	RecordID   string           `json:"id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserID     int              `json:"user_id" gorm:"not null;index:idx_user_source,priority:1"`
	TokenID    int              `json:"token_id" gorm:"index"`
	SourceHash string           `json:"-" gorm:"type:varchar(64);index:idx_user_source,priority:2"`
	SourceKind ImageAuditSource `json:"source_kind" gorm:"type:varchar(16)"`
	// PublicURL is the URL we actually handed to ARK CreateAsset. For
	// SourceKindURL it equals the user input; for SourceKindBase64 it's
	// the post-COS-upload public URL — internal infra detail, never
	// surfaced to clients (json:"-").
	PublicURL string           `json:"-" gorm:"type:varchar(2048)"`
	Project   string           `json:"project,omitempty" gorm:"type:varchar(64)"`
	GroupID   string           `json:"group_id,omitempty" gorm:"type:varchar(64)"`
	AssetID   string           `json:"asset_id,omitempty" gorm:"type:varchar(64);index"`
	AssetURI  string           `json:"asset_uri,omitempty" gorm:"type:varchar(96)"`
	Status    ImageAuditStatus `json:"status" gorm:"type:varchar(16);index"`
	Reason    string           `json:"reason,omitempty" gorm:"type:text"`
	CreatedAt int64            `json:"created_at" gorm:"index"`
	UpdatedAt int64            `json:"updated_at"`
}

// TableName pins the table name regardless of GORM naming strategy.
func (ImageAuditRecord) TableName() string { return "image_audit_records" }

// Touch updates UpdatedAt to now() — call before Save() so the sweeper
// can tell live pollers from orphaned ones.
func (r *ImageAuditRecord) Touch() {
	r.UpdatedAt = time.Now().Unix()
}
