package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	moneypricing "github.com/QuantumNous/new-api/service/money_pricing"
	"github.com/gin-gonic/gin"
)

type moneyPricingQuotePreviewRequest struct {
	Policy             model.RetailPricingPolicy       `json:"policy"`
	Features           moneypricing.MoneyUsageFeatures `json:"features"`
	SettlementCurrency string                          `json:"settlement_currency"`
}

func ListMoneyPricingFxRates(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	rates, total, err := model.ListFxRates(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(rates)
	common.ApiSuccess(c, pageInfo)
}

func CreateMoneyPricingFxRate(c *gin.Context) {
	var rate model.FxRate
	if err := common.DecodeJson(c.Request.Body, &rate); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if strings.TrimSpace(rate.Id) == "" || strings.TrimSpace(rate.BaseCurrency) == "" || strings.TrimSpace(rate.QuoteCurrency) == "" || rate.RateMicros <= 0 {
		common.ApiErrorMsg(c, "id、base_currency、quote_currency 和 rate_micros 不能为空")
		return
	}
	if err := model.DB.Create(&rate).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rate)
}

func UpdateMoneyPricingFxRate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.ApiErrorMsg(c, "无效的汇率 ID")
		return
	}
	existing, err := model.GetFxRateById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var incoming model.FxRate
	if err := common.DecodeJson(c.Request.Body, &incoming); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	incoming.Id = existing.Id
	incoming.CreatedAt = existing.CreatedAt
	if err := model.DB.Save(&incoming).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, incoming)
}

func DeleteMoneyPricingFxRate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.ApiErrorMsg(c, "无效的汇率 ID")
		return
	}
	if err := model.DeleteFxRateById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListMoneyPricingChannelModelCosts(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	costs, total, err := model.ListChannelModelCosts(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(costs)
	common.ApiSuccess(c, pageInfo)
}

func CreateMoneyPricingChannelModelCost(c *gin.Context) {
	var cost model.ChannelModelCost
	if err := common.DecodeJson(c.Request.Body, &cost); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	cost.Id = 0
	if err := model.DB.Create(&cost).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cost)
}

func UpdateMoneyPricingChannelModelCost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的渠道模型成本 ID")
		return
	}
	existing, err := model.GetChannelModelCostById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var incoming model.ChannelModelCost
	if err := common.DecodeJson(c.Request.Body, &incoming); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	incoming.Id = existing.Id
	incoming.CreatedAt = existing.CreatedAt
	if err := model.DB.Save(&incoming).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, incoming)
}

func DeleteMoneyPricingChannelModelCost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的渠道模型成本 ID")
		return
	}
	if err := model.DeleteChannelModelCostById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListMoneyPricingRetailPolicies(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	policies, total, err := model.ListRetailPricingPolicies(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(policies)
	common.ApiSuccess(c, pageInfo)
}

func CreateMoneyPricingRetailPolicy(c *gin.Context) {
	var policy model.RetailPricingPolicy
	if err := common.DecodeJson(c.Request.Body, &policy); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	policy.Id = 0
	if err := model.DB.Create(&policy).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, policy)
}

func UpdateMoneyPricingRetailPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的零售定价策略 ID")
		return
	}
	existing, err := model.GetRetailPricingPolicyById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var incoming model.RetailPricingPolicy
	if err := common.DecodeJson(c.Request.Body, &incoming); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	incoming.Id = existing.Id
	incoming.CreatedAt = existing.CreatedAt
	if err := model.DB.Save(&incoming).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, incoming)
}

func DeleteMoneyPricingRetailPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的零售定价策略 ID")
		return
	}
	if err := model.DeleteRetailPricingPolicyById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func PreviewMoneyPricingQuote(c *gin.Context) {
	var request moneyPricingQuotePreviewRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	quote, err := moneypricing.QuoteRetailPricingPolicy(&request.Policy, request.Features, request.SettlementCurrency)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, quote)
}
