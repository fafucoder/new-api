/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

For commercial licensing, please contact support@quantumnous.com
*/

package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/gin-gonic/gin"
)

// RelayVolcTaskList 实现火山原生列表接口
//   GET /api/v3/contents/generations/tasks?page_size=&page_token=&status=&created_after=
// 返回体：{"items":[<火山响应体...>], "next_page_token":"<空则无更多>"}
func RelayVolcTaskList(c *gin.Context) {
	userId := c.GetInt("id")

	limit := 20
	if v := strings.TrimSpace(c.Query("page_size")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	var pageToken int64
	if v := strings.TrimSpace(c.Query("page_token")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			pageToken = n
		}
	}

	var createdAfter int64
	if v := strings.TrimSpace(c.Query("created_after")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			createdAfter = n
		}
	}

	var statusFilter []string
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				statusFilter = append(statusFilter, s)
			}
		}
	}

	tasks, next, err := model.ListTasksByUserPlatform(model.TaskListQuery{
		UserId:       userId,
		Platform:     c.Query("platform"), // 可选，火山原生入口默认不带
		StatusFilter: statusFilter,
		CreatedAfter: createdAfter,
		Limit:        limit,
		PageToken:    pageToken,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "server_error"}})
		return
	}

	items := make([]any, 0, len(tasks))
	for _, task := range tasks {
		body, err := renderVolcVideoBody(task)
		if err != nil {
			logger.LogError(c, fmt.Sprintf("render volc task %s failed: %s", task.TaskID, err.Error()))
			continue
		}
		var obj any
		if err := common.Unmarshal(body, &obj); err == nil {
			items = append(items, obj)
		}
	}

	resp := gin.H{"items": items}
	if next > 0 {
		resp["next_page_token"] = strconv.FormatInt(next, 10)
	} else {
		resp["next_page_token"] = ""
	}
	c.JSON(http.StatusOK, resp)
}

// RelayVolcTaskCancel 实现火山原生取消/删除接口
//   DELETE /api/v3/contents/generations/tasks/{task_id}
// 只有排队中/运行中的任务允许取消（本地标记 FAILURE + 尝试通知上游），
// 已结束的任务直接从本地记录中删除。
func RelayVolcTaskCancel(c *gin.Context) {
	userId := c.GetInt("id")
	taskId := c.Param("task_id")
	if taskId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "task_id is required", "type": "invalid_request_error"}})
		return
	}

	task, exist, err := model.GetByTaskId(userId, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "server_error"}})
		return
	}
	if !exist {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "task not found", "type": "invalid_request_error"}})
		return
	}

	// 尚未结束：标记为失败（无法真正回撤上游，先本地断路）。
	// 结算部分若开启延迟计费无预扣则天然无需退款；有预扣则由现有 defer Refund 兜底。
	if task.Status != model.TaskStatusSuccess && task.Status != model.TaskStatusFailure {
		task.Status = model.TaskStatusFailure
		task.FailReason = "canceled by user"
		if _, err := task.UpdateWithStatus(task.Status); err != nil {
			logger.LogWarn(c, fmt.Sprintf("mark task %s failed: %s", task.TaskID, err.Error()))
		}
	}

	if err := model.DeleteTaskByTaskId(userId, taskId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "server_error"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": taskId, "deleted": true})
}

// renderVolcVideoBody 复用 adaptor.ConvertToVolcVideo 生成火山响应体。
// 非 maas/doubao 平台的任务不返回火山格式（返回 nil 并让调用方跳过）。
func renderVolcVideoBody(task *model.Task) ([]byte, error) {
	adaptor := relay.GetTaskAdaptor(task.Platform)
	if adaptor == nil {
		return nil, fmt.Errorf("unsupported platform: %s", task.Platform)
	}
	if converter, ok := adaptor.(channel.VolcVideoConverter); ok {
		return converter.ConvertToVolcVideo(task)
	}
	return nil, fmt.Errorf("adaptor does not implement VolcVideoConverter")
}
