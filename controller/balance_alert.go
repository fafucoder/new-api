// Package controller — balance_alert: 余额监控规则 CRUD + 手动测试。
//
// 规则按 Channel.Tag 聚合上游账户;界面展示每条规则的当前余额、
// 状态(正常 / 告警中)、上次告警时间。手动测试端点用来验证
// webhook 配置是否能正常送达。
package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// balanceAlertRuleView 是规则 + 实时聚合余额拼出来的视图,给前端
// 列表渲染。规则字段是 BalanceAlertRule 的快照;Summary 是当前
// 余额聚合,可能为 nil(tag 下还没有渠道时)。
type balanceAlertRuleView struct {
	model.BalanceAlertRule
	Summary  *model.TagBalanceSummary `json:"summary,omitempty"`
	Channels []balanceChannelBrief    `json:"channels,omitempty"`
}

type balanceChannelBrief struct {
	Id                 int     `json:"id"`
	Name               string  `json:"name"`
	Type               int     `json:"type"`
	Status             int     `json:"status"`
	Balance            float64 `json:"balance"`
	UsedQuota          int64   `json:"used_quota"`
	BalanceUpdatedTime int64   `json:"balance_updated_time"`
}

// GetBalanceAlertRules 返回全部规则 + 每条规则当前聚合的余额和
// 渠道清单。规则数量级很小(=上游账户数),所以一次性返回够用,
// 前端不需要分页。
func GetBalanceAlertRules(c *gin.Context) {
	rules, err := model.ListBalanceAlertRules()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]balanceAlertRuleView, 0, len(rules))
	for _, r := range rules {
		view := balanceAlertRuleView{BalanceAlertRule: r}
		if summary, err := model.AggregateBalanceForTag(r.Tag); err == nil {
			view.Summary = summary
		}
		if channels, err := model.ListChannelsForTag(r.Tag); err == nil {
			brief := make([]balanceChannelBrief, 0, len(channels))
			for _, ch := range channels {
				brief = append(brief, balanceChannelBrief{
					Id:                 ch.Id,
					Name:               ch.Name,
					Type:               ch.Type,
					Status:             ch.Status,
					Balance:            ch.Balance,
					UsedQuota:          ch.UsedQuota,
					BalanceUpdatedTime: ch.BalanceUpdatedTime,
				})
			}
			view.Channels = brief
		}
		views = append(views, view)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    views,
	})
}

// PostBalanceAlertRule 创建新规则。Body 直接绑 BalanceAlertRule,
// 但运行时字段(LastBalance / LastAlertedAt / LastCheckedAt /
// AlertState)由扫描任务维护,这里忽略客户端传入。
func PostBalanceAlertRule(c *gin.Context) {
	var rule model.BalanceAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		common.ApiError(c, err)
		return
	}
	rule.Id = 0
	rule.LastBalance = 0
	rule.LastAlertedAt = 0
	rule.LastCheckedAt = 0
	rule.AlertState = model.BalanceAlertStateNormal
	if err := model.CreateBalanceAlertRule(&rule); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rule,
	})
}

// PutBalanceAlertRule 更新规则的可编辑字段。
func PutBalanceAlertRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid rule id"})
		return
	}
	var rule model.BalanceAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		common.ApiError(c, err)
		return
	}
	rule.Id = id
	if err := model.UpdateBalanceAlertRule(&rule); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, _ := model.GetBalanceAlertRule(id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    updated,
	})
}

// DeleteBalanceAlertRuleHandler 删除规则。
func DeleteBalanceAlertRuleHandler(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid rule id"})
		return
	}
	if err := model.DeleteBalanceAlertRule(id); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// PostBalanceAlertRuleTopup 充值:把 amount 累加到 rule.TotalQuota,
// 并把 alert_state 拨回 normal,这样"充值后再跌破"能立即重新告警,
// 不被冷却抑制。
func PostBalanceAlertRuleTopup(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid rule id"})
		return
	}
	var body struct {
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ApiError(c, err)
		return
	}
	if body.Amount <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "充值金额必须大于 0"})
		return
	}
	if err := model.TopupBalanceAlertRule(id, body.Amount); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, _ := model.GetBalanceAlertRule(id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    updated,
	})
}

// GetBalanceAlertTags 返回 channels 表里所有非空 tag,给前端
// 新建规则时下拉选择。
func GetBalanceAlertTags(c *gin.Context) {
	tags, err := model.ListChannelTags()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    tags,
	})
}

// PostBalanceAlertRuleTest 立即按规则当前阈值触发一次告警,
// 不论实际余额是否低于阈值。用来在管理界面验证 webhook 是否
// 接得到。不更新 LastAlertedAt(测试不算正式触发)。
func PostBalanceAlertRuleTest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid rule id"})
		return
	}
	rule, err := model.GetBalanceAlertRule(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "rule not found"})
		return
	}

	summary, sErr := model.AggregateBalanceForTag(rule.Tag)
	if sErr != nil {
		summary = &model.TagBalanceSummary{Tag: rule.Tag}
	}
	balance := 0.0
	if summary != nil {
		balance = summary.Balance
	}

	if err := service.SendBalanceAlert(rule, balance, true); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "测试发送失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已发送测试告警",
	})
}
