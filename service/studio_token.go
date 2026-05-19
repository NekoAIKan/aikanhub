package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const StudioTokenName = "Studio Default"

func GetOrCreateStudioToken(userID int, group string) (*model.Token, error) {
	if userID <= 0 {
		return nil, errors.New("user id is required")
	}

	var token model.Token
	err := model.DB.Where("user_id = ? AND source = ? AND managed_by_system = ?", userID, model.TokenSourceStudio, true).
		Order("id asc").
		First(&token).Error
	exists, err := model.RecordExist(err)
	if err != nil {
		return nil, err
	}
	if exists {
		if token.Status != common.TokenStatusEnabled || !token.UnlimitedQuota || token.Group != group || token.PublicApiEnabled {
			token.Status = common.TokenStatusEnabled
			token.UnlimitedQuota = true
			token.ExpiredTime = -1
			token.Group = group
			token.PublicApiEnabled = false
			if err := token.Update(); err != nil {
				return nil, err
			}
		}
		return &token, nil
	}

	key, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}

	token = model.Token{
		UserId:           userID,
		Name:             StudioTokenName,
		Key:              key,
		Status:           common.TokenStatusEnabled,
		CreatedTime:      common.GetTimestamp(),
		AccessedTime:     common.GetTimestamp(),
		ExpiredTime:      -1,
		UnlimitedQuota:   true,
		Group:            group,
		Source:           model.TokenSourceStudio,
		PublicApiEnabled: false,
		ManagedBySystem:  true,
	}
	if err := token.Insert(); err != nil {
		return nil, err
	}
	return &token, nil
}
