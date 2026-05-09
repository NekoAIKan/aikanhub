// image_audit_error.go — map *imageaudit.PerImageAuditError into a
// TaskError with structured failed_image data, so multi-image audit
// rejections tell the client exactly which image to replace.
//
// Lives in the relay package (not service) to keep the imageaudit
// service free of dto/relay imports.
package relay

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service/imageaudit"
)

// taskErrorFromImageAuditError walks err's chain looking for a
// *PerImageAuditError. When found, returns a TaskError with:
//
//   - Code: image_audit_failed (moderation rejection) or
//     image_audit_error (infra failure)
//   - StatusCode: 400 for moderation, 502 for infra
//   - Data: { failed_image: {index, role, source, asset_id, reason} }
//
// Source is truncated to a sane length (URL or short data URI tag) so
// a multi-MB base64 input doesn't bloat the error response.
//
// Returns nil if err doesn't wrap a PerImageAuditError — caller falls
// back to the generic build_request_failed wrapper.
func taskErrorFromImageAuditError(err error) *dto.TaskError {
	var perImg *imageaudit.PerImageAuditError
	if !errors.As(err, &perImg) {
		return nil
	}

	failedImage := map[string]any{
		"index":  perImg.Index,
		"source": truncateAuditSource(perImg.Source),
	}
	if perImg.Role != "" {
		failedImage["role"] = perImg.Role
	}
	if id := perImg.AssetID(); id != "" {
		failedImage["asset_id"] = id
	}
	if reason := perImg.Reason(); reason != "" {
		failedImage["reason"] = reason
	}

	code := "image_audit_error"
	status := http.StatusBadGateway
	if imageaudit.IsAuditFailure(err) {
		code = "image_audit_failed"
		status = http.StatusBadRequest
	}

	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: status,
		Data:       map[string]any{"failed_image": failedImage},
		Error:      err,
		LocalError: true,
	}
}

// truncateAuditSource keeps URLs intact (they're useful for the
// client) and replaces base64 data URIs with their MIME prefix only,
// so the error stays small.
func truncateAuditSource(src string) string {
	const maxURL = 256
	if len(src) <= maxURL {
		return src
	}
	// data:image/jpeg;base64,... — keep the header for diagnostics,
	// drop the payload.
	if len(src) > 5 && src[:5] == "data:" {
		// Find first comma; everything before is the descriptor.
		for i := 0; i < len(src) && i < 64; i++ {
			if src[i] == ',' {
				return src[:i+1] + "...(base64 payload elided)"
			}
		}
	}
	return src[:maxURL] + "...(truncated)"
}
