// audit_test.go — unit tests for the cache + asset:// passthrough
// branches of Submit. ARK-network paths (createAsset, ensureAssetGroup,
// pollAsset) are exercised in the container e2e test, not here.
package imageaudit

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("open test db: " + err.Error())
	}
	if err := db.AutoMigrate(&model.ImageAuditRecord{}); err != nil {
		panic("migrate: " + err.Error())
	}
	model.DB = db
	common.UsingSQLite = true

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	os.Exit(m.Run())
}

func resetTable(t *testing.T) {
	t.Helper()
	model.DB.Exec("DELETE FROM image_audit_records")
}

func TestHashSource_Stable(t *testing.T) {
	a := hashSource("https://example.com/foo.jpg")
	b := hashSource("https://example.com/foo.jpg")
	c := hashSource("  https://example.com/foo.jpg  ") // whitespace trimmed
	d := hashSource("https://example.com/bar.jpg")
	assert.Equal(t, a, b)
	assert.Equal(t, a, c, "leading/trailing whitespace must not change the hash")
	assert.NotEqual(t, a, d)
	assert.Len(t, a, 64, "sha256 hex = 64 chars")
}

func TestSourceKindOf(t *testing.T) {
	assert.Equal(t, model.ImageAuditSourceURL, sourceKindOf("http://x"))
	assert.Equal(t, model.ImageAuditSourceURL, sourceKindOf("https://x"))
	assert.Equal(t, model.ImageAuditSourceBase64, sourceKindOf("data:image/png;base64,iVBOR..."))
	assert.Equal(t, model.ImageAuditSourceBase64, sourceKindOf("iVBOR..."))
}

func TestSubmit_RejectsEmpty(t *testing.T) {
	resetTable(t)
	_, err := Submit(context.Background(), "", 1, 0)
	assert.Error(t, err)

	_, err = Submit(context.Background(), "   ", 1, 0)
	assert.Error(t, err)
}

func TestSubmit_AssetPassthrough_ScopedByUser(t *testing.T) {
	resetTable(t)
	// User 1 audited an image and got asset-foo. Persist that.
	owned := &model.ImageAuditRecord{
		UserID:   1,
		AssetID:  "asset-foo",
		AssetURI: "asset://asset-foo",
		Status:   model.ImageAuditStatusActive,
	}
	require.NoError(t, model.CreateImageAuditRecord(owned))

	// User 1 resubmits asset://asset-foo → returns the existing record.
	got, err := Submit(context.Background(), "asset://asset-foo", 1, 99)
	require.NoError(t, err)
	assert.Equal(t, owned.RecordID, got.RecordID, "user 1 must see their own record")

	// User 2 submits the same asset id — must NOT see user 1's record.
	got, err = Submit(context.Background(), "asset://asset-foo", 2, 99)
	require.NoError(t, err)
	assert.NotEqual(t, owned.RecordID, got.RecordID, "user 2 must not get user 1's record")
	assert.True(t, len(got.RecordID) > 9 && got.RecordID[:9] == "external_",
		"unknown asset for user 2 should be a synthetic external_ record, got %q", got.RecordID)
	assert.Equal(t, model.ImageAuditStatusActive, got.Status)
	assert.Equal(t, "asset://asset-foo", got.AssetURI)
}

func TestSubmit_AssetPassthrough_RejectsEmptyID(t *testing.T) {
	resetTable(t)
	_, err := Submit(context.Background(), "asset://", 1, 0)
	assert.Error(t, err)
}

func TestSubmit_CacheHitOnFailed_DoesNotReAudit(t *testing.T) {
	// The bug fix: a previously-failed audit should NOT trigger a new
	// ARK CreateAsset call. We force this by NOT setting ARK creds
	// (so any attempt to call ARK would error) and asserting Submit
	// returns the cached failed record cleanly.
	resetTable(t)

	src := "https://example.com/blocked-image.jpg"
	sh := hashSource(src)

	prior := &model.ImageAuditRecord{
		UserID:     42,
		SourceHash: sh,
		SourceKind: model.ImageAuditSourceURL,
		Status:     model.ImageAuditStatusFailed,
		Reason:     "moderation: contains restricted content",
		AssetID:    "asset-deadbeef",
	}
	require.NoError(t, model.CreateImageAuditRecord(prior))

	// Wipe ARK env so any unintended ARK call would error.
	t.Setenv("ARK_AK", "")
	t.Setenv("ARK_SK", "")

	got, err := Submit(context.Background(), src, 42, 0)
	require.NoError(t, err, "cached failed should be returned without re-audit")
	assert.Equal(t, model.ImageAuditStatusFailed, got.Status)
	assert.Equal(t, prior.RecordID, got.RecordID)
	assert.Equal(t, "moderation: contains restricted content", got.Reason)
}

func TestSubmit_CacheHitOnActive_DoesNotReAudit(t *testing.T) {
	resetTable(t)
	src := "https://example.com/known-good.jpg"
	prior := &model.ImageAuditRecord{
		UserID:     5,
		SourceHash: hashSource(src),
		SourceKind: model.ImageAuditSourceURL,
		Status:     model.ImageAuditStatusActive,
		AssetURI:   "asset://asset-good",
		AssetID:    "asset-good",
	}
	require.NoError(t, model.CreateImageAuditRecord(prior))

	t.Setenv("ARK_AK", "")
	t.Setenv("ARK_SK", "")

	got, err := Submit(context.Background(), src, 5, 0)
	require.NoError(t, err)
	assert.Equal(t, prior.RecordID, got.RecordID)
	assert.Equal(t, model.ImageAuditStatusActive, got.Status)
}

func TestSubmit_NoARK_ReturnsConfigError_OnFreshAudit(t *testing.T) {
	// Without ARK creds AND without a cached row, Submit must error
	// with the configuration message — not a confusing transient one.
	resetTable(t)
	t.Setenv("ARK_AK", "")
	t.Setenv("ARK_SK", "")

	_, err := Submit(context.Background(), "https://example.com/new.jpg", 7, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ARK_AK")
}

func TestEnsureAudited_AssetPassthrough_ReturnsURIDirectly(t *testing.T) {
	resetTable(t)
	// EnsureAudited on asset://... should not need ARK creds.
	t.Setenv("ARK_AK", "")
	t.Setenv("ARK_SK", "")

	uri, err := EnsureAudited(context.Background(), "asset://asset-x", 1, 0)
	require.NoError(t, err)
	assert.Equal(t, "asset://asset-x", uri)
}

func TestEnsureAudited_FailedCacheHit_ReturnsAuditError(t *testing.T) {
	resetTable(t)
	src := "https://example.com/bad.jpg"
	rec := &model.ImageAuditRecord{
		UserID:     8,
		SourceHash: hashSource(src),
		SourceKind: model.ImageAuditSourceURL,
		Status:     model.ImageAuditStatusFailed,
		Reason:     "moderation rejected",
		AssetID:    "asset-bad",
	}
	require.NoError(t, model.CreateImageAuditRecord(rec))

	_, err := EnsureAudited(context.Background(), src, 8, 0)
	require.Error(t, err)
	assert.True(t, IsAuditFailure(err), "failed cache hit must surface as AuditError")

	var ae *AuditError
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, "asset-bad", ae.AssetId)
	assert.Equal(t, "moderation rejected", ae.Reason)
}

func TestPerImageAuditError_UnwrapsToAuditError(t *testing.T) {
	inner := &AuditError{AssetId: "asset-y", Status: "Failed", Reason: "blocked"}
	per := &PerImageAuditError{Index: 2, Role: "first_frame", Source: "https://x", Err: inner}

	assert.True(t, IsAuditFailure(per), "IsAuditFailure should walk the chain")

	var ae *AuditError
	require.True(t, errors.As(per, &ae))
	assert.Equal(t, "asset-y", ae.AssetId)

	assert.Equal(t, "asset-y", per.AssetID())
	assert.Equal(t, "blocked", per.Reason())
}

func TestPerImageAuditError_NonAuditWrappedAsInfra(t *testing.T) {
	per := &PerImageAuditError{Index: 0, Source: "x", Err: errors.New("network unreachable")}
	assert.False(t, IsAuditFailure(per), "infra error must not be classified as audit failure")
	assert.Equal(t, "", per.AssetID())
	assert.Equal(t, "", per.Reason())
}
