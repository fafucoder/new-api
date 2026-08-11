package model

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssetLibraryGroup struct {
	Id          int64                      `json:"id" gorm:"primaryKey"`
	UserId      int                        `json:"-" gorm:"index;not null"`
	DisplayName string                     `json:"display_name" gorm:"type:varchar(64);not null"`
	Description string                     `json:"description" gorm:"type:text"`
	GroupType   string                     `json:"group_type" gorm:"type:varchar(32);default:'AIGC'"`
	CoverURL    string                     `json:"cover_url" gorm:"type:text"`
	CreatedTime int64                      `json:"created_time" gorm:"bigint;index"`
	UpdatedTime int64                      `json:"updated_time" gorm:"bigint;index"`
	Assets      []AssetLibraryAsset        `json:"assets" gorm:"foreignKey:GroupId"`
	Mappings    []AssetLibraryGroupChannel `json:"mappings" gorm:"foreignKey:GroupId"`
}

type AssetLibraryAsset struct {
	Id          int64                      `json:"id" gorm:"primaryKey"`
	GroupId     int64                      `json:"group_id" gorm:"index;not null"`
	UserId      int                        `json:"-" gorm:"index;not null"`
	Name        string                     `json:"name" gorm:"type:varchar(255);not null"`
	AssetType   string                     `json:"asset_type" gorm:"type:varchar(32);not null"`
	CreatedTime int64                      `json:"created_time" gorm:"bigint"`
	UpdatedTime int64                      `json:"updated_time" gorm:"bigint"`
	Mappings    []AssetLibraryAssetChannel `json:"mappings" gorm:"foreignKey:AssetId"`
}

type AssetLibraryGroupChannel struct {
	Id              int64  `json:"id" gorm:"primaryKey"`
	GroupId         int64  `json:"group_id" gorm:"uniqueIndex:idx_asset_group_channel;not null"`
	UserId          int    `json:"-" gorm:"index;not null"`
	ChannelId       int    `json:"channel_id" gorm:"uniqueIndex:idx_asset_group_channel;index;not null"`
	UpstreamGroupId string `json:"upstream_group_id" gorm:"type:varchar(255);index"`
	Status          string `json:"status" gorm:"type:varchar(32);not null"`
	ErrorMessage    string `json:"error_message" gorm:"type:text"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64  `json:"updated_time" gorm:"bigint"`
	ChannelName     string `json:"channel_name" gorm:"-"`
}

type AssetLibraryAssetChannel struct {
	Id              int64  `json:"id" gorm:"primaryKey"`
	AssetId         int64  `json:"asset_id" gorm:"uniqueIndex:idx_asset_channel;not null"`
	GroupChannelId  int64  `json:"group_channel_id" gorm:"index;not null"`
	UserId          int    `json:"-" gorm:"index;not null"`
	ChannelId       int    `json:"channel_id" gorm:"uniqueIndex:idx_asset_channel;index;not null"`
	UpstreamAssetId string `json:"upstream_asset_id" gorm:"type:varchar(255);index"`
	AssetURL        string `json:"asset_url" gorm:"type:text"`
	Status          string `json:"status" gorm:"type:varchar(32);not null"`
	ErrorCode       string `json:"error_code" gorm:"type:varchar(255)"`
	ErrorMessage    string `json:"error_message" gorm:"type:text"`
	CreatedTime     int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64  `json:"updated_time" gorm:"bigint"`
	ChannelName     string `json:"channel_name" gorm:"-"`
}

func ListAssetLibraryGroups(userId int) ([]AssetLibraryGroup, error) {
	var groups []AssetLibraryGroup
	err := DB.Where("user_id = ?", userId).
		Order("updated_time DESC, id DESC").
		Preload("Mappings").
		Preload("Assets", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Preload("Assets.Mappings").
		Find(&groups).Error
	return groups, err
}

func GetAssetLibraryGroup(userId int, id int64) (*AssetLibraryGroup, error) {
	var group AssetLibraryGroup
	err := DB.Where("id = ? AND user_id = ?", id, userId).
		Preload("Mappings").
		Preload("Assets", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Preload("Assets.Mappings").
		First(&group).Error
	return &group, err
}

func CreateAssetLibraryGroup(group *AssetLibraryGroup) error {
	now := time.Now().Unix()
	group.CreatedTime = now
	group.UpdatedTime = now
	for i := range group.Assets {
		group.Assets[i].UserId = group.UserId
		group.Assets[i].CreatedTime = now
		group.Assets[i].UpdatedTime = now
	}
	return DB.Create(group).Error
}

func CreateAssetLibraryAssets(assets []AssetLibraryAsset) error {
	if len(assets) == 0 {
		return nil
	}
	now := time.Now().Unix()
	for i := range assets {
		assets[i].CreatedTime = now
		assets[i].UpdatedTime = now
	}
	return DB.Create(&assets).Error
}

func SaveAssetLibraryGroupChannel(mapping *AssetLibraryGroupChannel) error {
	now := time.Now().Unix()
	if mapping.CreatedTime == 0 {
		mapping.CreatedTime = now
	}
	mapping.UpdatedTime = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group_id"}, {Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"upstream_group_id", "status", "error_message", "updated_time",
		}),
	}).Create(mapping).Error
}

func SaveAssetLibraryAssetChannel(mapping *AssetLibraryAssetChannel) error {
	now := time.Now().Unix()
	if mapping.CreatedTime == 0 {
		mapping.CreatedTime = now
	}
	mapping.UpdatedTime = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "asset_id"}, {Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"group_channel_id", "upstream_asset_id", "asset_url", "status",
			"error_code", "error_message", "updated_time",
		}),
	}).Create(mapping).Error
}

func UpdateAssetLibraryGroup(userId int, id int64, values map[string]interface{}) error {
	values["updated_time"] = time.Now().Unix()
	result := DB.Model(&AssetLibraryGroup{}).Where("id = ? AND user_id = ?", id, userId).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func UpdateAssetLibraryAsset(userId int, id int64, values map[string]interface{}) error {
	values["updated_time"] = time.Now().Unix()
	return DB.Model(&AssetLibraryAsset{}).Where("id = ? AND user_id = ?", id, userId).Updates(values).Error
}

func DeleteAssetLibraryGroup(userId int, id int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var assetIds []int64
		if err := tx.Model(&AssetLibraryAsset{}).Where("group_id = ? AND user_id = ?", id, userId).Pluck("id", &assetIds).Error; err != nil {
			return err
		}
		if len(assetIds) > 0 {
			if err := tx.Where("asset_id IN ?", assetIds).Delete(&AssetLibraryAssetChannel{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("group_id = ? AND user_id = ?", id, userId).Delete(&AssetLibraryAsset{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ? AND user_id = ?", id, userId).Delete(&AssetLibraryGroupChannel{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND user_id = ?", id, userId).Delete(&AssetLibraryGroup{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func DeleteAssetLibraryAsset(userId int, groupId int64, assetId int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("asset_id = ? AND user_id = ?", assetId, userId).Delete(&AssetLibraryAssetChannel{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND group_id = ? AND user_id = ?", assetId, groupId, userId).Delete(&AssetLibraryAsset{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&AssetLibraryGroup{}).Where("id = ? AND user_id = ?", groupId, userId).
			Update("updated_time", time.Now().Unix()).Error
	})
}

func GetAssetLibraryChannels() ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("status = ?", 1).Find(&channels).Error
	return channels, err
}
