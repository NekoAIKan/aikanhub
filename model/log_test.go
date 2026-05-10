package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecordTopupLogPersistsQuota guards the regression behind PR #55:
// RecordTopupLog used to write the topup amount only into Content and leave
// Log.Quota = 0, which made the frontend Cost cell render blank for topup
// rows in /usage-logs.
//
// If a future change drops the Quota field from the inserted row again, the
// frontend `+$X` chip will silently disappear without a compile error — this
// test is the canary.
func TestRecordTopupLogPersistsQuota(t *testing.T) {
	truncateTables(t)

	const userID = 9001
	const quotaAdded = 500_000 // tokens corresponding to $1 at QuotaPerUnit=500k

	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: "topup-test-user",
	}).Error)

	RecordTopupLog(
		userID,
		quotaAdded,
		"使用在线充值成功，充值金额: $1.00，支付金额：7.30",
		"127.0.0.1",
		"alipay",
		"epay",
	)

	var log Log
	require.NoError(t, DB.Where("user_id = ? AND type = ?", userID, LogTypeTopup).First(&log).Error)

	assert.Equal(t, quotaAdded, log.Quota,
		"topup log must persist the credited quota — frontend Cost cell renders the green +$X chip from this field")
	assert.Equal(t, LogTypeTopup, log.Type)
	assert.Equal(t, userID, log.UserId)
	assert.NotEmpty(t, log.Content, "content should still carry the human-readable summary")
}
