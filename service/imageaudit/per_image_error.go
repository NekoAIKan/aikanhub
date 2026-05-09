// per_image_error.go — structured error for the multi-image audit path.
// Lets the upstream error envelope tell clients exactly which image in
// a batch caused the rejection (issue #43 acceptance criteria).
package imageaudit

import (
	"errors"
	"fmt"
)

// PerImageAuditError annotates an audit error with positional context.
// The wrapped Err is either *AuditError (moderation rejection) or a
// plain error (infra failure). Use IsAuditFailure(err) on the wrapped
// chain to distinguish them.
//
// JSON shape consumed by the response builder:
//
//	{
//	  "code": "image_audit_failed",   // or "image_audit_error"
//	  "failed_image": {
//	    "index": 2,
//	    "role": "first_frame",
//	    "source": "https://...",
//	    "asset_id": "asset-...",       // present when ARK assigned one
//	    "reason":   "..."              // ARK reason, when available
//	  }
//	}
type PerImageAuditError struct {
	// Index is the position in the upstream content[] array (0-based).
	Index int
	// Role is the optional content role: "first_frame", "last_frame",
	// "reference_image", "reference_video", or "" for default.
	Role string
	// Source is the original image input string the user supplied.
	// May be a URL or a base64 data URI; we don't truncate base64 here
	// so the controller's response builder can decide.
	Source string
	// Err is the underlying audit error.
	Err error
}

func (e *PerImageAuditError) Error() string {
	if e == nil {
		return ""
	}
	role := e.Role
	if role == "" {
		role = "(none)"
	}
	return fmt.Sprintf("image[%d] role=%s: %v", e.Index, role, e.Err)
}

// Unwrap allows errors.As / errors.Is to traverse to the underlying
// AuditError. The relay error formatter relies on this.
func (e *PerImageAuditError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// AssetID returns the ARK asset id from the wrapped AuditError if any.
// Empty when the wrapped error is an infra failure (no asset was ever
// assigned).
func (e *PerImageAuditError) AssetID() string {
	if e == nil {
		return ""
	}
	var ae *AuditError
	if errors.As(e.Err, &ae) {
		return ae.AssetId
	}
	return ""
}

// Reason returns the ARK reason text from the wrapped AuditError if
// any. Empty for infra failures.
func (e *PerImageAuditError) Reason() string {
	if e == nil {
		return ""
	}
	var ae *AuditError
	if errors.As(e.Err, &ae) {
		return ae.Reason
	}
	return ""
}
