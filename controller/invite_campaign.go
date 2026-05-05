package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetAllInviteCampaigns(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	campaigns, total, err := model.GetAllInviteCampaigns(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(campaigns)
	common.ApiSuccess(c, pageInfo)
}

func GetInviteCampaign(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的邀请码活动 ID")
		return
	}
	campaign, err := model.GetInviteCampaignById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, campaign)
}

func AddInviteCampaign(c *gin.Context) {
	var campaign model.InviteCampaign
	if err := common.DecodeJson(c.Request.Body, &campaign); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if strings.TrimSpace(campaign.Code) == "" || strings.TrimSpace(campaign.Name) == "" {
		common.ApiErrorMsg(c, "邀请码和名称不能为空")
		return
	}
	if err := campaign.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    campaign,
	})
}

func UpdateInviteCampaign(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的邀请码活动 ID")
		return
	}
	var incoming model.InviteCampaign
	if err := common.DecodeJson(c.Request.Body, &incoming); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	campaign, err := model.GetInviteCampaignById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	campaign.Code = incoming.Code
	campaign.Name = incoming.Name
	campaign.Status = incoming.Status
	campaign.StartTime = incoming.StartTime
	campaign.EndTime = incoming.EndTime
	campaign.UsageLimit = incoming.UsageLimit
	campaign.Quota = incoming.Quota
	campaign.Group = incoming.Group
	if err := campaign.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, campaign)
}

func DeleteInviteCampaign(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的邀请码活动 ID")
		return
	}
	if err := model.DeleteInviteCampaignById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
