package model

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// truncateImageAudit clears the table between subtests. Sibling tables
// from task_cas_test.go's truncateTables are unaffected.
func truncateImageAudit(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM image_audit_records")
	})
}

func TestNewImageAuditRecordID_FormatAndUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewImageAuditRecordID()
		assert.True(t, len(id) >= len(recordIDPrefix)+8, "id too short: %s", id)
		assert.Equal(t, recordIDPrefix, id[:len(recordIDPrefix)])
		assert.False(t, seen[id], "duplicate id generated: %s", id)
		seen[id] = true
	}
}

func TestCreateImageAuditRecord_AssignsIDAndTimestamps(t *testing.T) {
	truncateImageAudit(t)
	r := &ImageAuditRecord{
		UserID:     1,
		SourceHash: "h1",
		Status:     ImageAuditStatusProcessing,
	}
	require.NoError(t, CreateImageAuditRecord(r))
	assert.NotEmpty(t, r.RecordID)
	assert.NotZero(t, r.CreatedAt)
	assert.NotZero(t, r.UpdatedAt)
}

func TestFindActiveImageAuditByHash_OnlyReturnsActive(t *testing.T) {
	truncateImageAudit(t)
	for _, st := range []ImageAuditStatus{
		ImageAuditStatusProcessing,
		ImageAuditStatusFailed,
	} {
		r := &ImageAuditRecord{UserID: 1, SourceHash: "shx", Status: st}
		require.NoError(t, CreateImageAuditRecord(r))
	}
	_, err := FindActiveImageAuditByHash(1, "shx")
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound),
		"expected ErrRecordNotFound when no active row exists, got %v", err)

	active := &ImageAuditRecord{UserID: 1, SourceHash: "shx", Status: ImageAuditStatusActive, AssetURI: "asset://a1"}
	require.NoError(t, CreateImageAuditRecord(active))

	got, err := FindActiveImageAuditByHash(1, "shx")
	require.NoError(t, err)
	assert.Equal(t, "asset://a1", got.AssetURI)
}

func TestFindTerminalImageAuditByHash_PrefersActiveOverFailed(t *testing.T) {
	truncateImageAudit(t)
	// Insert failed first, then active. Active should win because
	// FindTerminalImageAuditByHash first tries the active query.
	failed := &ImageAuditRecord{UserID: 7, SourceHash: "h", Status: ImageAuditStatusFailed, Reason: "old fail"}
	require.NoError(t, CreateImageAuditRecord(failed))

	got, err := FindTerminalImageAuditByHash(7, "h")
	require.NoError(t, err)
	assert.Equal(t, ImageAuditStatusFailed, got.Status, "with only failed row, should return it")

	active := &ImageAuditRecord{UserID: 7, SourceHash: "h", Status: ImageAuditStatusActive, AssetURI: "asset://winner"}
	require.NoError(t, CreateImageAuditRecord(active))

	got, err = FindTerminalImageAuditByHash(7, "h")
	require.NoError(t, err)
	assert.Equal(t, ImageAuditStatusActive, got.Status, "active row should be preferred")
	assert.Equal(t, "asset://winner", got.AssetURI)
}

func TestFindTerminalImageAuditByHash_ScopedByUser(t *testing.T) {
	truncateImageAudit(t)
	// User 1's row should not be visible to user 2 even though hash matches.
	r := &ImageAuditRecord{UserID: 1, SourceHash: "shared", Status: ImageAuditStatusActive, AssetURI: "asset://u1"}
	require.NoError(t, CreateImageAuditRecord(r))

	_, err := FindTerminalImageAuditByHash(2, "shared")
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound),
		"user 2 must not see user 1's records, got %v", err)
}

func TestGetImageAuditRecordByRecordID_Scoping(t *testing.T) {
	truncateImageAudit(t)
	r := &ImageAuditRecord{UserID: 5, SourceHash: "h", Status: ImageAuditStatusActive}
	require.NoError(t, CreateImageAuditRecord(r))

	// Wrong user → not found.
	_, err := GetImageAuditRecordByRecordID(99, r.RecordID)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))

	// Correct user → found.
	got, err := GetImageAuditRecordByRecordID(5, r.RecordID)
	require.NoError(t, err)
	assert.Equal(t, r.RecordID, got.RecordID)

	// userID=0 bypasses scope (used by background poller).
	got, err = GetImageAuditRecordByRecordID(0, r.RecordID)
	require.NoError(t, err)
	assert.Equal(t, r.RecordID, got.RecordID)
}

func TestListImageAuditRecords_PaginationAndOrdering(t *testing.T) {
	truncateImageAudit(t)
	for i := 0; i < 25; i++ {
		r := &ImageAuditRecord{UserID: 3, SourceHash: "h", Status: ImageAuditStatusActive}
		require.NoError(t, CreateImageAuditRecord(r))
	}

	rows, total, err := ListImageAuditRecords(3, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 25, total)
	assert.Len(t, rows, 10)

	// Newest first: id of first row should be > last row.
	assert.Greater(t, rows[0].ID, rows[len(rows)-1].ID)

	// Page 3 has 5 rows.
	rows, _, err = ListImageAuditRecords(3, 3, 10)
	require.NoError(t, err)
	assert.Len(t, rows, 5)

	// pageSize clamp: 0 → default 20; >100 → 100.
	rows, _, err = ListImageAuditRecords(3, 1, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 20)
	rows, _, err = ListImageAuditRecords(3, 1, 9999)
	require.NoError(t, err)
	assert.Len(t, rows, 25, "all 25 fit under the 100 cap")
}

func TestFindStaleProcessingImageAudits(t *testing.T) {
	truncateImageAudit(t)
	now := time.Now()

	// Two processing rows, one fresh (within window), one stale.
	stale := &ImageAuditRecord{UserID: 1, SourceHash: "stale", Status: ImageAuditStatusProcessing}
	require.NoError(t, CreateImageAuditRecord(stale))
	DB.Model(&ImageAuditRecord{}).Where("id = ?", stale.ID).
		Update("updated_at", now.Add(-30*time.Minute).Unix())

	fresh := &ImageAuditRecord{UserID: 1, SourceHash: "fresh", Status: ImageAuditStatusProcessing}
	require.NoError(t, CreateImageAuditRecord(fresh))

	// Active row mustn't be returned even if old.
	old := &ImageAuditRecord{UserID: 1, SourceHash: "old", Status: ImageAuditStatusActive}
	require.NoError(t, CreateImageAuditRecord(old))
	DB.Model(&ImageAuditRecord{}).Where("id = ?", old.ID).
		Update("updated_at", now.Add(-1*time.Hour).Unix())

	// Cutoff at 5 minutes ago.
	rows, err := FindStaleProcessingImageAudits(now.Add(-5 * time.Minute))
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, stale.RecordID, rows[0].RecordID)
}

func TestImageAuditStatus_IsTerminal(t *testing.T) {
	assert.True(t, ImageAuditStatusActive.IsTerminal())
	assert.True(t, ImageAuditStatusFailed.IsTerminal())
	assert.False(t, ImageAuditStatusProcessing.IsTerminal())
}

func TestImageAuditRecord_PublicURLNotInJSON(t *testing.T) {
	// PublicURL is internal infra detail; never marshal it.
	r := &ImageAuditRecord{
		RecordID:  "imgaudit_test",
		UserID:    1,
		PublicURL: "https://internal.cos/secret-bucket/abc.jpg",
		Status:    ImageAuditStatusActive,
	}
	b, err := json.Marshal(r)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "secret-bucket",
		"PublicURL must not appear in JSON output")
	assert.NotContains(t, string(b), "public_url")
}
