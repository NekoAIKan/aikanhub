// refresh_test.go — unit coverage for RefreshFromUpstream's local
// branches. The ARK network path (200/Active/Failed payloads) is
// exercised end-to-end via image_audit_e2e.py against a running
// container; pulling in a transport mock here would over-couple the
// test to internal HTTP wiring.
package imageaudit

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshFromUpstream_NilRecord(t *testing.T) {
	// Defensive: nil input must error, not panic. Catches a future
	// caller who skips the gorm.ErrRecordNotFound check.
	_, err := RefreshFromUpstream(context.Background(), nil)
	assert.Error(t, err)
}

func TestRefreshFromUpstream_TerminalActive_NoChange(t *testing.T) {
	// Terminal rows are authoritative — no ARK call, no DB write. We
	// prove "no ARK call" indirectly by leaving ARK_AK/SK empty; if
	// the code tried to call ARK it would short-circuit on HasARK
	// anyway, but the assertion below confirms the row is byte-for-
	// byte unchanged.
	resetTable(t)
	t.Setenv("ARK_AK", "")
	t.Setenv("ARK_SK", "")

	rec := &model.ImageAuditRecord{
		UserID:   1,
		AssetID:  "asset-good",
		AssetURI: "asset://asset-good",
		Status:   model.ImageAuditStatusActive,
	}
	require.NoError(t, model.CreateImageAuditRecord(rec))
	originalUpdated := rec.UpdatedAt

	got, err := RefreshFromUpstream(context.Background(), rec)
	require.NoError(t, err)
	assert.Equal(t, model.ImageAuditStatusActive, got.Status)
	assert.Equal(t, originalUpdated, got.UpdatedAt,
		"terminal record must not be re-saved (UpdatedAt unchanged)")
}

func TestRefreshFromUpstream_TerminalFailed_NoChange(t *testing.T) {
	resetTable(t)
	t.Setenv("ARK_AK", "")
	t.Setenv("ARK_SK", "")

	rec := &model.ImageAuditRecord{
		UserID:  2,
		AssetID: "asset-bad",
		Status:  model.ImageAuditStatusFailed,
		Reason:  "moderation rejected",
	}
	require.NoError(t, model.CreateImageAuditRecord(rec))

	got, err := RefreshFromUpstream(context.Background(), rec)
	require.NoError(t, err)
	assert.Equal(t, model.ImageAuditStatusFailed, got.Status)
	assert.Equal(t, "moderation rejected", got.Reason,
		"failed verdict must be preserved verbatim")
}

func TestRefreshFromUpstream_ProcessingNoARK_ReturnsAsIs(t *testing.T) {
	// Without ARK creds we can't ask upstream — must NOT fail the
	// read. Return the input row unchanged so the client sees its
	// processing state and can retry later.
	resetTable(t)
	t.Setenv("ARK_AK", "")
	t.Setenv("ARK_SK", "")

	rec := &model.ImageAuditRecord{
		UserID:  3,
		AssetID: "asset-pending",
		Status:  model.ImageAuditStatusProcessing,
	}
	require.NoError(t, model.CreateImageAuditRecord(rec))

	got, err := RefreshFromUpstream(context.Background(), rec)
	require.NoError(t, err)
	assert.Equal(t, model.ImageAuditStatusProcessing, got.Status,
		"no ARK creds → row returned unchanged, not failed")
}

func TestRefreshFromUpstream_ProcessingNoAssetID_ReturnsAsIs(t *testing.T) {
	// Synthetic / pre-persist records may carry no AssetID. Refresh
	// must short-circuit; there's nothing to ask ARK about.
	resetTable(t)
	t.Setenv("ARK_AK", "ak")
	t.Setenv("ARK_SK", "sk")

	rec := &model.ImageAuditRecord{
		UserID: 4,
		Status: model.ImageAuditStatusProcessing,
	}
	got, err := RefreshFromUpstream(context.Background(), rec)
	require.NoError(t, err)
	assert.Equal(t, model.ImageAuditStatusProcessing, got.Status)
}
