package controller

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var allowedProxyTypes = map[string]bool{
	"http":   true,
	"https":  true,
	"socks5": true,
}

func validateProxyURL(raw string) error {
	if raw == "" {
		return errors.New("url required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid proxy url")
	}
	if u.Scheme == "" || u.Host == "" {
		return errors.New("invalid proxy url")
	}
	return nil
}

func GetAllProxies(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	keyword := c.Query("keyword")
	statusFilter, _ := strconv.Atoi(c.DefaultQuery("status", "0"))

	items, total, err := model.ListProxies(page, size, keyword, statusFilter)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"items": items, "total": total, "page": page, "size": size},
	})
}

func GetProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	p, err := model.GetProxyById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "proxy not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

func AddProxy(c *gin.Context) {
	var p model.Proxy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !allowedProxyTypes[p.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid type"})
		return
	}
	if err := validateProxyURL(p.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	p.Id = 0
	if err := p.Insert(); err != nil {
		if errors.Is(err, model.ErrProxyNameConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "proxy name already exists"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.InvalidateProxyCache()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

func UpdateProxy(c *gin.Context) {
	var p model.Proxy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if p.Id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "id required"})
		return
	}
	if !allowedProxyTypes[p.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid type"})
		return
	}
	if err := validateProxyURL(p.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := p.Update(); err != nil {
		if errors.Is(err, model.ErrProxyNameConflict) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "proxy name already exists"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.InvalidateProxyCache()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	channels, err := model.ListChannelsByProxyId(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if len(channels) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"success":             false,
			"message":             "proxy is referenced by channels, please unbind first",
			"referenced_channels": channels,
		})
		return
	}
	p := &model.Proxy{Id: id}
	if err := p.Delete(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	model.InvalidateProxyCache()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func TestProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	p, err := model.GetProxyById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "proxy not found"})
		return
	}
	target := p.TestURL
	if target == "" {
		target = common.DefaultProxyTestURL
	}
	ok, latency, msg := service.TestProxyConnectivity(p.URL, target, 10*time.Second)
	_ = model.UpdateProxyTestResult(id, ok, msg)
	model.InvalidateProxyCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"ok": ok, "latency_ms": latency, "msg": msg, "target": target},
	})
}

func GetProxyReferences(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}
	channels, err := model.ListChannelsByProxyId(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": channels})
}

func GetProxyOptions(c *gin.Context) {
	onlyEnabled := c.DefaultQuery("only_enabled", "true") == "true"
	statusFilter := 0
	if onlyEnabled {
		statusFilter = model.ProxyStatusEnabled
	}
	items, _, err := model.ListProxies(1, 200, "", statusFilter)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	options := make([]gin.H, 0, len(items))
	for _, p := range items {
		options = append(options, gin.H{
			"id":     p.Id,
			"name":   p.Name,
			"type":   p.Type,
			"status": p.Status,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": options})
}
