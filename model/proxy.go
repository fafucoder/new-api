package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	ProxyStatusEnabled  = 1
	ProxyStatusDisabled = 2
)

type Proxy struct {
	Id           int    `json:"id"`
	Name         string `json:"name" gorm:"type:varchar(64);uniqueIndex;not null"`
	Type         string `json:"type" gorm:"type:varchar(16);not null"`
	URL          string `json:"url" gorm:"type:varchar(512);not null"`
	TestURL      string `json:"test_url" gorm:"type:varchar(512);default:''"`
	Description  string `json:"description" gorm:"type:varchar(255);default:''"`
	Status       int    `json:"status" gorm:"default:1"`
	LastTestTime int64  `json:"last_test_time" gorm:"bigint;default:0"`
	LastTestOK   bool   `json:"last_test_ok" gorm:"default:false"`
	LastTestMsg  string `json:"last_test_msg" gorm:"type:varchar(255);default:''"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
}

var ErrProxyNameConflict = errors.New("proxy name already exists")

func (p *Proxy) Insert() error {
	if p.Name == "" {
		return errors.New("proxy name required")
	}
	if p.URL == "" {
		return errors.New("proxy url required")
	}
	p.CreatedTime = time.Now().Unix()
	if p.Status == 0 {
		p.Status = ProxyStatusEnabled
	}
	err := DB.Create(p).Error
	if err != nil && isUniqueConstraintError(err) {
		return ErrProxyNameConflict
	}
	return err
}

func (p *Proxy) Update() error {
	if p.Id <= 0 {
		return errors.New("proxy id required")
	}
	err := DB.Model(&Proxy{}).Where("id = ?", p.Id).Updates(map[string]interface{}{
		"name":        p.Name,
		"type":        p.Type,
		"url":         p.URL,
		"test_url":    p.TestURL,
		"description": p.Description,
		"status":      p.Status,
	}).Error
	if err != nil && isUniqueConstraintError(err) {
		return ErrProxyNameConflict
	}
	return err
}

func (p *Proxy) Delete() error {
	if p.Id <= 0 {
		return errors.New("proxy id required")
	}
	return DB.Delete(p).Error
}

func GetProxyById(id int) (*Proxy, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var p Proxy
	err := DB.First(&p, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetProxyByName(name string) (*Proxy, error) {
	var p Proxy
	err := DB.First(&p, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func ListProxies(page int, size int, keyword string, statusFilter int) ([]*Proxy, int64, error) {
	if page < 1 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	q := DB.Model(&Proxy{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name LIKE ? OR url LIKE ? OR description LIKE ?", like, like, like)
	}
	if statusFilter != 0 {
		q = q.Where("status = ?", statusFilter)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*Proxy
	err := q.Order("id DESC").Limit(size).Offset((page - 1) * size).Find(&items).Error
	return items, total, err
}

func UpdateProxyTestResult(id int, ok bool, msg string) error {
	updates := map[string]interface{}{
		"last_test_time": time.Now().Unix(),
		"last_test_ok":   ok,
		"last_test_msg":  msg,
	}
	return DB.Model(&Proxy{}).Where("id = ?", id).Updates(updates).Error
}

func CountChannelsByProxyId(proxyId int) (int64, error) {
	if proxyId <= 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&Channel{}).Where("proxy_id = ?", proxyId).Count(&count).Error
	return count, err
}

func ListChannelsByProxyId(proxyId int) ([]*Channel, error) {
	if proxyId <= 0 {
		return nil, nil
	}
	var channels []*Channel
	err := DB.Select("id, name, type, status").Where("proxy_id = ?", proxyId).Find(&channels).Error
	return channels, err
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, k := range []string{"UNIQUE", "unique", "Duplicate", "duplicate"} {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}
