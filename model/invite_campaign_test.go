package model

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupInviteCampaignTestDB(t *testing.T) {
	t.Helper()
	ensureModelTestSchema(t, &InviteCampaign{})
	require.NoError(t, DB.Exec("DELETE FROM invite_campaigns").Error)
	t.Cleanup(func() {
		DB.Exec("DELETE FROM invite_campaigns")
	})
}

func TestInviteCampaignConsumeHappyPathIncrementsUsage(t *testing.T) {
	setupInviteCampaignTestDB(t)
	now := int64(2000)
	require.NoError(t, DB.Create(&InviteCampaign{
		Code:       "BETA2026",
		Name:       "Beta launch",
		Status:     InviteCampaignStatusEnabled,
		StartTime:  now - 100,
		EndTime:    now + 100,
		UsageLimit: 2,
		Quota:      9000,
		Group:      "beta",
	}).Error)

	campaign, err := ConsumeInviteCampaignCode("BETA2026", now)
	require.NoError(t, err)
	require.Equal(t, 9000, campaign.Quota)
	require.Equal(t, "beta", campaign.Group)

	var stored InviteCampaign
	require.NoError(t, DB.First(&stored, "code = ?", "BETA2026").Error)
	require.Equal(t, 1, stored.UsedCount)
}

func TestInviteCampaignConsumeRejectsExpiredAndExhaustedCampaigns(t *testing.T) {
	setupInviteCampaignTestDB(t)
	now := int64(2000)
	require.NoError(t, DB.Create(&InviteCampaign{
		Code:      "OLD",
		Name:      "Old",
		Status:    InviteCampaignStatusEnabled,
		StartTime: now - 200,
		EndTime:   now - 1,
		Quota:     100,
	}).Error)
	require.NoError(t, DB.Create(&InviteCampaign{
		Code:       "FULL",
		Name:       "Full",
		Status:     InviteCampaignStatusEnabled,
		UsageLimit: 1,
		UsedCount:  1,
		Quota:      100,
	}).Error)

	_, err := ConsumeInviteCampaignCode("OLD", now)
	require.ErrorIs(t, err, ErrInviteCampaignExpired)

	_, err = ConsumeInviteCampaignCode("FULL", now)
	require.ErrorIs(t, err, ErrInviteCampaignExhausted)

	var count int64
	require.NoError(t, DB.Model(&InviteCampaign{}).Where("used_count > ?", 1).Count(&count).Error)
	require.Zero(t, count)
}

func TestInviteCampaignConsumeEnforcesUsageLimitOnIncrement(t *testing.T) {
	setupInviteCampaignTestDB(t)
	now := int64(2000)
	require.NoError(t, DB.Create(&InviteCampaign{
		Code:       "ONCE",
		Name:       "One use",
		Status:     InviteCampaignStatusEnabled,
		UsageLimit: 1,
		Quota:      100,
	}).Error)

	_, err := ConsumeInviteCampaignCode("ONCE", now)
	require.NoError(t, err)

	_, err = ConsumeInviteCampaignCode("ONCE", now)
	require.ErrorIs(t, err, ErrInviteCampaignExhausted)

	var stored InviteCampaign
	require.NoError(t, DB.First(&stored, "code = ?", "ONCE").Error)
	require.Equal(t, 1, stored.UsedCount)
}

func TestInviteCampaignConsumeDistinguishesDisabledFromMissing(t *testing.T) {
	setupInviteCampaignTestDB(t)
	require.NoError(t, DB.Create(&InviteCampaign{
		Code:   "OFF",
		Name:   "Disabled",
		Status: InviteCampaignStatusDisabled,
		Quota:  100,
	}).Error)

	_, err := ConsumeInviteCampaignCode("OFF", 2000)
	require.True(t, errors.Is(err, ErrInviteCampaignDisabled))

	_, err = ConsumeInviteCampaignCode("MISSING", 2000)
	require.True(t, errors.Is(err, ErrInviteCampaignNotFound))
}

func TestInviteCampaignConsumeConcurrentLimitAllowsOneSuccess(t *testing.T) {
	setupInviteCampaignTestDB(t)
	now := int64(2000)
	require.NoError(t, DB.Create(&InviteCampaign{
		Code:       "ONCEPARALLEL",
		Name:       "One parallel use",
		Status:     InviteCampaignStatusEnabled,
		UsageLimit: 1,
		Quota:      100,
	}).Error)

	var successCount atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ConsumeInviteCampaignCode("ONCEPARALLEL", now)
			if err == nil {
				successCount.Add(1)
			} else {
				require.ErrorIs(t, err, ErrInviteCampaignExhausted)
			}
		}()
	}
	wg.Wait()

	require.Equal(t, int64(1), successCount.Load())
	var stored InviteCampaign
	require.NoError(t, DB.First(&stored, "code = ?", "ONCEPARALLEL").Error)
	require.Equal(t, 1, stored.UsedCount)
}
