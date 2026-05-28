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
//   - Code:
//     image_audit_failed   — moderation verdict, 400
//     image_audit_rejected — upstream 4xx (e.g. width > 6000px), 400
//     image_audit_error    — true infra failure, 502
//   - StatusCode: 4xx for caller-fixable cases, 502 only when we genuinely
//     failed as a gateway. Status MUST be 4xx for upstream user-input
//     rejections because Cloudflare substitutes its own HTML page for any
//     source 5xx, hiding our JSON body (and the upstream's actionable hint).
//   - Message: when the upstream returned a structured Code/Message, we use
//     its Message verbatim so callers see "Width must be between 300px and
//     6000px." rather than our wrapper chain.
//   - Data: { failed_image: {...}, upstream: {code, http} }
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
	message := err.Error()

	var ark *imageaudit.ArkUpstreamError
	switch {
	case imageaudit.IsAuditFailure(err):
		code = "image_audit_failed"
		status = http.StatusBadRequest
	case errors.As(err, &ark) && ark.IsClientFault():
		code = "image_audit_rejected"
		status = http.StatusBadRequest
		// Surface the upstream's own message verbatim; falling back to the
		// truncated raw body keeps the caller informed even when Volcano
		// returns an unparseable envelope.
		if ark.MetaMsg != "" {
			message = ark.MetaMsg
		} else if ark.RawBody != "" {
			message = ark.RawBody
		}
	}

	data := map[string]any{"failed_image": failedImage}
	if ark != nil {
		upstream := map[string]any{"http": ark.HTTPStatus}
		if ark.MetaCode != "" {
			upstream["code"] = ark.MetaCode
		}
		data["upstream"] = upstream
	}

	return &dto.TaskError{
		Code:       code,
		Message:    message,
		StatusCode: status,
		Data:       data,
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
