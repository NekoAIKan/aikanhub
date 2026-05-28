package relay

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/service/imageaudit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskErrorFromImageAuditError_ArkClientFault_PassesThrough4xxAndUpstreamMessage
// guards Rule 31 / Issue #68-style regressions: when ARK rejects a caller's
// reference image because the upstream validation fails (HTTP 4xx +
// InvalidParameter.*), the gateway must surface a 4xx with the upstream's own
// Message as the visible error, not a generic 502 + wrapper chain. CF
// substitutes its own HTML for any source 5xx, so a 502 here would hide the
// actionable hint from the caller.
func TestTaskErrorFromImageAuditError_ArkClientFault_PassesThrough4xxAndUpstreamMessage(t *testing.T) {
	ark := &imageaudit.ArkUpstreamError{
		Action:     "CreateAsset",
		HTTPStatus: http.StatusBadRequest,
		MetaCode:   "InvalidParameter.WidthTooLarge",
		MetaMsg:    "Width must be between 300px and 6000px.",
		RawBody:    `{"ResponseMetadata":{"Error":{...}}}`,
	}
	// Mirror the production wrap chain: Submit wraps with fmt.Errorf %w and
	// the relay layer wraps in *PerImageAuditError before reaching here.
	wrapped := &imageaudit.PerImageAuditError{
		Index:  0,
		Role:   "reference_image",
		Source: "https://example.r2.cloudflarestorage.com/seedvr2_upscaled.jpg",
		Err:    fmt.Errorf("imageaudit: createAsset: %w", ark),
	}

	got := taskErrorFromImageAuditError(wrapped)
	require.NotNil(t, got, "expected a TaskError, got nil")

	assert.Equal(t, http.StatusBadRequest, got.StatusCode,
		"upstream 4xx must propagate as 4xx, not 502 — CF eats 5xx bodies")
	assert.Equal(t, "image_audit_rejected", got.Code)
	assert.Equal(t, "Width must be between 300px and 6000px.", got.Message,
		"caller must see the upstream's own message verbatim")

	data, ok := got.Data.(map[string]any)
	require.True(t, ok)
	upstream, ok := data["upstream"].(map[string]any)
	require.True(t, ok, "data.upstream should expose the upstream code/status")
	assert.Equal(t, "InvalidParameter.WidthTooLarge", upstream["code"])
	assert.Equal(t, http.StatusBadRequest, upstream["http"])

	failedImage, ok := data["failed_image"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0, failedImage["index"])
	assert.Equal(t, "reference_image", failedImage["role"])
}

// TestTaskErrorFromImageAuditError_AuditFailure_Stays400 guards the existing
// audit-moderation branch — a deterministic content-rejection from ARK must
// still emit image_audit_failed + 400 (not the new image_audit_rejected).
func TestTaskErrorFromImageAuditError_AuditFailure_Stays400(t *testing.T) {
	wrapped := &imageaudit.PerImageAuditError{
		Index:  1,
		Role:   "first_frame",
		Source: "https://x/y.jpg",
		Err:    &imageaudit.AuditError{AssetId: "asset-abc", Status: "Failed", Reason: "policy_violation"},
	}
	got := taskErrorFromImageAuditError(wrapped)
	require.NotNil(t, got)

	assert.Equal(t, http.StatusBadRequest, got.StatusCode)
	assert.Equal(t, "image_audit_failed", got.Code)
}

// TestTaskErrorFromImageAuditError_InfraFailure_Stays502 guards that a
// non-typed, non-AuditError chain still falls into the 502 default — that's
// the legitimate "we are the gateway and we genuinely failed" case (network
// error talking to ARK, panic, etc.).
func TestTaskErrorFromImageAuditError_InfraFailure_Stays502(t *testing.T) {
	wrapped := &imageaudit.PerImageAuditError{
		Index:  0,
		Source: "https://x/y.jpg",
		Err:    errors.New("dial tcp: connection refused"),
	}
	got := taskErrorFromImageAuditError(wrapped)
	require.NotNil(t, got)

	assert.Equal(t, http.StatusBadGateway, got.StatusCode)
	assert.Equal(t, "image_audit_error", got.Code)
}

// TestTaskErrorFromImageAuditError_NonImageAuditError_ReturnsNil keeps the
// caller-side fallback intact — callers (relay_task.go) only switch into the
// per-image envelope when this function returns non-nil; an arbitrary error
// must continue to fall through to the generic build_request_failed wrapper.
func TestTaskErrorFromImageAuditError_NonImageAuditError_ReturnsNil(t *testing.T) {
	assert.Nil(t, taskErrorFromImageAuditError(errors.New("some other error")))
}
