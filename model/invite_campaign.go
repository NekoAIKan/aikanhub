package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	InviteCampaignStatusEnabled  = 1
	InviteCampaignStatusDisabled = 2
)

var (
	ErrInviteCampaignNotFound  = errors.New("无效的邀请码")
	ErrInviteCampaignExpired   = errors.New("邀请码已过期")
	ErrInviteCampaignExhausted = errors.New("邀请码已用完")
)

type InviteCampaign struct {
	Id          int            `json:"id"`
	Code        string         `json:"code" gorm:"type:varchar(64);uniqueIndex"`
	Name        string         `json:"name" gorm:"type:varchar(128);index"`
	Status      int            `json:"status" gorm:"type:int;default:1"`
	StartTime   int64          `json:"start_time" gorm:"bigint;default:0"`
	EndTime     int64          `json:"end_time" gorm:"bigint;default:0"`
	UsageLimit  int            `json:"usage_limit" gorm:"type:int;default:0"`
	UsedCount   int            `json:"used_count" gorm:"type:int;default:0"`
	Quota       int            `json:"quota" gorm:"type:int;default:-1"`
	Group       string         `json:"group" gorm:"type:varchar(64)"`
	CreatedTime int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func normalizeInviteCampaignCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func (campaign *InviteCampaign) normalize() {
	campaign.Code = normalizeInviteCampaignCode(campaign.Code)
	campaign.Name = strings.TrimSpace(campaign.Name)
	campaign.Group = strings.TrimSpace(campaign.Group)
	now := common.GetTimestamp()
	if campaign.CreatedTime == 0 {
		campaign.CreatedTime = now
	}
	campaign.UpdatedTime = now
	if campaign.Status == 0 {
		campaign.Status = InviteCampaignStatusEnabled
	}
	if campaign.Quota < -1 {
		campaign.Quota = -1
	}
}

func (campaign *InviteCampaign) Insert() error {
	campaign.normalize()
	return DB.Create(campaign).Error
}

func (campaign *InviteCampaign) Update() error {
	campaign.normalize()
	return DB.Model(campaign).Select("code", "name", "status", "start_time", "end_time", "usage_limit", "quota", "group", "updated_time").Updates(campaign).Error
}

func DeleteInviteCampaignById(id int) error {
	return DB.Delete(&InviteCampaign{}, id).Error
}

func GetInviteCampaignById(id int) (*InviteCampaign, error) {
	var campaign InviteCampaign
	err := DB.First(&campaign, "id = ?", id).Error
	return &campaign, err
}

func GetAllInviteCampaigns(startIdx int, num int) ([]*InviteCampaign, int64, error) {
	var campaigns []*InviteCampaign
	var total int64
	tx := DB.Model(&InviteCampaign{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&campaigns).Error; err != nil {
		return nil, 0, err
	}
	return campaigns, total, nil
}

func ConsumeInviteCampaignCode(code string, now int64) (*InviteCampaign, error) {
	var consumed *InviteCampaign
	err := DB.Transaction(func(tx *gorm.DB) error {
		campaign, err := ConsumeInviteCampaignCodeWithTx(tx, code, now)
		if err != nil {
			return err
		}
		consumed = campaign
		return nil
	})
	return consumed, err
}

func ConsumeInviteCampaignCodeWithTx(tx *gorm.DB, code string, now int64) (*InviteCampaign, error) {
	code = normalizeInviteCampaignCode(code)
	if code == "" {
		return nil, ErrInviteCampaignNotFound
	}

	var campaign InviteCampaign
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("code = ? AND status = ?", code, InviteCampaignStatusEnabled).
		First(&campaign).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInviteCampaignNotFound
		}
		return nil, err
	}
	if campaign.StartTime != 0 && campaign.StartTime > now {
		return nil, ErrInviteCampaignExpired
	}
	if campaign.EndTime != 0 && campaign.EndTime < now {
		return nil, ErrInviteCampaignExpired
	}
	if campaign.UsageLimit > 0 && campaign.UsedCount >= campaign.UsageLimit {
		return nil, ErrInviteCampaignExhausted
	}

	result := tx.Model(&InviteCampaign{}).
		Where("id = ? AND (usage_limit <= ? OR used_count < usage_limit)", campaign.Id, 0).
		Updates(map[string]any{
			"used_count":   gorm.Expr("used_count + ?", 1),
			"updated_time": now,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrInviteCampaignExhausted
	}
	campaign.UsedCount++
	campaign.UpdatedTime = now
	return &campaign, nil
}
