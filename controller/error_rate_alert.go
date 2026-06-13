package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetErrorRateAlerts 获取所有错误率告警规则
func GetErrorRateAlerts(c *gin.Context) {
	rules, err := model.ListErrorRateAlertRules()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rules,
	})
}

// GetErrorRateAlert 获取单条规则
func GetErrorRateAlert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	rule, err := model.GetErrorRateAlertRule(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if rule == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "rule not found",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rule,
	})
}

// CreateErrorRateAlert 创建错误率告警规则
func CreateErrorRateAlert(c *gin.Context) {
	var rule model.ErrorRateAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err := model.CreateErrorRateAlertRule(&rule); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rule,
	})
}

// UpdateErrorRateAlert 更新错误率告警规则
func UpdateErrorRateAlert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	var rule model.ErrorRateAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	rule.Id = id
	if err := model.UpdateErrorRateAlertRule(&rule); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// DeleteErrorRateAlert 删除错误率告警规则
func DeleteErrorRateAlert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	if err := model.DeleteErrorRateAlertRule(id); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// TestErrorRateAlert 测试错误率告警规则的 webhook 推送
func TestErrorRateAlert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "invalid id",
		})
		return
	}
	rule, err := model.GetErrorRateAlertRule(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if rule == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "rule not found",
		})
		return
	}

	// 构造测试数据
	testResult := &model.ErrorRateResult{
		ErrorCount:   10,
		SuccessCount: 90,
		Total:        100,
		ErrorRate:    10.0,
	}

	if err := service.SendErrorRateAlertTest(rule, testResult); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "发送测试告警失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "测试告警已发送",
	})
}
