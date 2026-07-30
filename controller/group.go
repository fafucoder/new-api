package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupType := c.Query("type")
	typeMap := ratio_setting.GetGroupTypeCopy()

	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if groupType == "" {
			groupNames = append(groupNames, groupName)
			continue
		}
		// 未分类的分组默认视为 channel
		t := typeMap[groupName]
		if t == "" {
			t = "channel"
		}
		if t == groupType {
			groupNames = append(groupNames, groupName)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	isAdmin := c.GetInt("role") >= 10
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserSelectableGroups(userGroup)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if desc, ok := userUsableGroups[groupName]; ok {
			info := map[string]interface{}{"desc": desc}
			if isAdmin {
				info["ratio"] = service.GetUserGroupRatio(userGroup, groupName)
			}
			usableGroups[groupName] = info
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		info := map[string]interface{}{"desc": setting.GetUsableGroupDescription("auto")}
		if isAdmin {
			info["ratio"] = "自动"
		}
		usableGroups["auto"] = info
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
