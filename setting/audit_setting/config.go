// Package audit_setting holds the runtime-configurable knobs for the
// full-request audit-logging subsystem (middleware/audit_log.go +
// service/auditlog).
//
// Storage model (operator decision): audit records are NOT written to the
// database. Each record is one JSON line appended to a local file under the
// log dir. The active file is rotated by size or age; on rotation the
// sealed file is uploaded to COS as a SINGLE object (one object per
// rotated file — never one per request) and then removed locally once the
// upload is confirmed. The active (un-rotated) file always stays local.
// This keeps the hot path off the DB, off per-request object storage, and
// off Neon migration entirely.
package audit_setting

import "github.com/QuantumNous/new-api/setting/config"

type AuditSetting struct {
	// Enabled gates the whole subsystem. When false the middleware
	// early-returns before wrapping the writer — zero overhead.
	Enabled bool `json:"enabled"`

	CaptureRequestBody  bool `json:"capture_request_body"`
	CaptureResponseBody bool `json:"capture_response_body"`

	// Redact masks Authorization + known secret JSON keys before the line
	// is written. Strongly recommended on.
	Redact     bool     `json:"redact"`
	RedactKeys []string `json:"redact_keys"`

	// MaxBodyBytes caps each captured body inside one JSON line. Larger
	// bodies are stored as a bounded prefix + a truncation marker (full
	// pixels of a base64 image are not forensically interesting; the
	// request params are, and they are tiny). No per-request object
	// storage — this is a single inline cap.
	MaxBodyBytes int64 `json:"max_body_bytes"`

	// StreamingFirstChunkBytes: SSE/streaming responses are never buffered
	// whole; only this many leading bytes are kept for debugging.
	StreamingFirstChunkBytes int64 `json:"streaming_first_chunk_bytes"`

	// SampleRate in [0,1]; 1 = every request.
	SampleRate float64 `json:"sample_rate"`

	// PathSkipPrefixes: paths to skip (health checks, static, the audit
	// read API itself).
	PathSkipPrefixes []string `json:"path_skip_prefixes"`

	// --- rotation / archival ---

	// RotateMaxBytes: seal + rotate the active file once it grows past
	// this. RotateIntervalMinutes: also rotate after this much wall-clock
	// even if the file is small (bounds how long a record sits only in the
	// un-archived active file). Either trigger fires rotation.
	RotateMaxBytes        int64 `json:"rotate_max_bytes"`
	RotateIntervalMinutes int   `json:"rotate_interval_minutes"`

	// ArchiveToCOS: on rotation, upload the sealed file to COS as one
	// object, then delete it locally once the upload is confirmed. When
	// false (or COS unconfigured) sealed files stay local and are GC'd by
	// LocalRetentionHours.
	ArchiveToCOS bool `json:"archive_to_cos"`

	// LocalRetentionHours: GC ceiling for sealed files. A sealed file that
	// has NOT been successfully archived is never deleted while
	// ArchiveToCOS is true (durability beats disk) — this only caps files
	// that are already in COS, or all sealed files when ArchiveToCOS is
	// false. 0 disables local GC.
	LocalRetentionHours int `json:"local_retention_hours"`
}

// defaultAuditSetting — operator chose "enabled by default, full capture".
// Safety rails (redaction on, body cap, streaming-safe, bounded rotation,
// local GC) keep audit from taking the gateway down.
var defaultAuditSetting = AuditSetting{
	Enabled:             true,
	CaptureRequestBody:  true,
	CaptureResponseBody: true,
	Redact:              true,
	RedactKeys: []string{
		"authorization", "api_key", "apikey", "api-key", "key",
		"token", "access_token", "refresh_token", "secret",
		"client_secret", "password", "passwd", "private_key",
	},
	MaxBodyBytes:             64 * 1024, // 64 KB per body per line
	StreamingFirstChunkBytes: 8 * 1024,
	SampleRate:               1.0,
	PathSkipPrefixes: []string{
		"/api/status", "/api/audit", "/health", "/healthz",
		"/favicon", "/static/", "/assets/", "/pwa/",
	},
	RotateMaxBytes:        64 * 1024 * 1024, // 64 MB
	RotateIntervalMinutes: 60,
	ArchiveToCOS:          true,
	LocalRetentionHours:   72,
}

func init() {
	config.GlobalConfig.Register("audit_setting", &defaultAuditSetting)
}

// GetAuditSetting returns the live, admin-mutated settings instance.
func GetAuditSetting() *AuditSetting {
	return &defaultAuditSetting
}
