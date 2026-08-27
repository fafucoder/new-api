package model

import (
	"time"

	"gorm.io/gorm"
)

// Asset library upstream formats.
const (
	AssetLibraryFormatVolcengine = "volcengine"
	AssetLibraryFormatOpenAI     = "openai"
)

// AssetLibraryUpstream is an administrator-configured destination that client
// assets are pushed to. It is deliberately decoupled from Channel: asset library
// authentication and endpoints are managed here, not on relay channels.
//
// A "volcengine" upstream speaks the query-action protocol (asset groups +
// CreateAsset). An "openai" upstream speaks the OpenAI Files API (POST /v1/files
// returning a file id; no group concept).
type AssetLibraryUpstream struct {
	Id          int64  `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"type:varchar(128);not null"`
	Format      string `json:"format" gorm:"type:varchar(32);not null;default:'volcengine'"`
	BaseURL     string `json:"base_url" gorm:"type:varchar(512);not null"`
	APIKey      string `json:"api_key" gorm:"type:text"`
	Enabled     bool   `json:"enabled" gorm:"default:true"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`

	// volcengine-specific
	Version           string `json:"version" gorm:"type:varchar(32)"`
	ProjectName       string `json:"project_name" gorm:"type:varchar(128)"`
	ListGroupsAction  string `json:"list_groups_action" gorm:"type:varchar(64)"`
	CreateGroupAction string `json:"create_group_action" gorm:"type:varchar(64)"`
	GetGroupAction    string `json:"get_group_action" gorm:"type:varchar(64)"`
	UpdateGroupAction string `json:"update_group_action" gorm:"type:varchar(64)"`
	DeleteGroupAction string `json:"delete_group_action" gorm:"type:varchar(64)"`
	CreateAssetAction string `json:"create_asset_action" gorm:"type:varchar(64)"`
	GetAssetAction    string `json:"get_asset_action" gorm:"type:varchar(64)"`
	UpdateAssetAction string `json:"update_asset_action" gorm:"type:varchar(64)"`
	DeleteAssetAction string `json:"delete_asset_action" gorm:"type:varchar(64)"`

	// openai-specific
	Purpose string `json:"purpose" gorm:"type:varchar(64)"`
}

func ListAssetLibraryUpstreams() ([]AssetLibraryUpstream, error) {
	var upstreams []AssetLibraryUpstream
	err := DB.Order("id ASC").Find(&upstreams).Error
	return upstreams, err
}

// ListEnabledAssetLibraryUpstreams returns only enabled upstreams, used when
// pushing assets.
func ListEnabledAssetLibraryUpstreams() ([]AssetLibraryUpstream, error) {
	var upstreams []AssetLibraryUpstream
	err := DB.Where("enabled = ?", true).Order("id ASC").Find(&upstreams).Error
	return upstreams, err
}

func GetAssetLibraryUpstream(id int64) (*AssetLibraryUpstream, error) {
	var upstream AssetLibraryUpstream
	err := DB.Where("id = ?", id).First(&upstream).Error
	if err != nil {
		return nil, err
	}
	return &upstream, nil
}

func CreateAssetLibraryUpstream(upstream *AssetLibraryUpstream) error {
	now := time.Now().Unix()
	upstream.CreatedTime = now
	upstream.UpdatedTime = now
	return DB.Create(upstream).Error
}

func UpdateAssetLibraryUpstream(id int64, values map[string]interface{}) error {
	values["updated_time"] = time.Now().Unix()
	result := DB.Model(&AssetLibraryUpstream{}).Where("id = ?", id).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeleteAssetLibraryUpstream(id int64) error {
	result := DB.Where("id = ?", id).Delete(&AssetLibraryUpstream{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
