// Package controller — invoice: 发票申请 / 列表 / 管理员审核 handler。
//
// 路由分组在 router/api-router.go:
//
//	/api/invoice/*       (登录用户)
//	/api/invoice/admin/* (管理员)
package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	invoicesvc "github.com/QuantumNous/new-api/service/invoice"

	"github.com/gin-gonic/gin"
)

// GetInvoiceSummary 余额卡片 + 申请按钮状态用。
func GetInvoiceSummary(c *gin.Context) {
	userID := c.GetInt("id")
	view, err := invoicesvc.Summary(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": view})
}

// GetInvoiceList 用户自己的发票历史, 分页。
func GetInvoiceList(c *gin.Context) {
	userID := c.GetInt("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := model.ListInvoicesForUser(userID, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"items": items, "total": total,
	}})
}

// PostInvoiceApply 用户提交一次开票申请, 申请金额由 service 决定。
func PostInvoiceApply(c *gin.Context) {
	userID := c.GetInt("id")
	var req dto.InvoiceApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	id, err := invoicesvc.Apply(userID, invoicesvc.ApplyForm{
		ApplicantType: req.ApplicantType,
		Title:         req.Title,
		TaxID:         req.TaxID,
		Email:         req.Email,
		InvoiceType:   req.InvoiceType,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": id}})
}

// GetInvoiceAdminList 管理员侧带筛选的发票列表。
func GetInvoiceAdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	filter := model.InvoiceListFilter{}
	if s := c.QueryArray("status"); len(s) > 0 {
		filter.Statuses = s
	}
	if uid, err := strconv.Atoi(c.Query("user_id")); err == nil && uid > 0 {
		filter.UserID = uid
	}
	if v, err := strconv.ParseInt(c.Query("applied_from"), 10, 64); err == nil {
		filter.AppliedFrom = v
	}
	if v, err := strconv.ParseInt(c.Query("applied_to"), 10, 64); err == nil {
		filter.AppliedTo = v
	}
	items, total, err := model.AdminListInvoices(filter, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"items": items, "total": total,
	}})
}

// PostInvoiceIssue 管理员手动触发开票。
func PostInvoiceIssue(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	reviewerID := c.GetInt("id")
	if err := invoicesvc.Issue(id, reviewerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PostInvoiceReject 管理员拒绝 pending 申请并填写原因。
func PostInvoiceReject(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	var req dto.InvoiceRejectRequest
	_ = c.ShouldBindJSON(&req)
	reviewerID := c.GetInt("id")
	if err := invoicesvc.Reject(id, reviewerID, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
