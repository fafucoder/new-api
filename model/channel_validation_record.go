// Package model — channel_validation_record persists the outcome of
// each Claude authenticity probe run.
//
// The record table is kept deliberately small: the summary fields
// (verdict / ok / model / duration) drive the history list UI and
// filtering, while the full ValidationResult JSON is stored in
// result_json for the detail panel. Both admins and regular users
// produce records; visibility is enforced at the controller layer.
package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ChannelValidationRecord is one row per validation suite run. We
// store result_json as TEXT (cross-DB safe) instead of JSONB so the
// table works on SQLite/MySQL/PostgreSQL without dialect-specific
// migrations. Result_json is omitted from list responses to keep
// payloads small; clients fetch a single record by id to inspect.
type ChannelValidationRecord struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	UserId      int    `json:"user_id" gorm:"index;index:idx_cv_user_time,priority:1"`
	Username    string `json:"username" gorm:"type:varchar(64)"`
	ChannelId   int    `json:"channel_id" gorm:"index"`
	ChannelName string `json:"channel_name" gorm:"type:varchar(128)"`
	Model       string `json:"model" gorm:"type:varchar(128);index"`
	Verdict     string `json:"verdict" gorm:"type:varchar(32);index"`
	OK          bool   `json:"ok"`
	DurationMs  int64  `json:"duration_ms"`
	Summary     string `json:"summary" gorm:"type:varchar(512)"`
	ResultJson  string `json:"result_json,omitempty" gorm:"type:text"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;index;index:idx_cv_user_time,priority:2,sort:desc"`
}

func (ChannelValidationRecord) TableName() string {
	return "channel_validation_records"
}

const channelValidationSummaryMax = 500

// CreateChannelValidationRecord inserts a new record. The caller is
// expected to fill in all summary fields plus result_json; the helper
// only enforces the summary length cap + created_time default.
func CreateChannelValidationRecord(record *ChannelValidationRecord) error {
	if record == nil {
		return errors.New("nil record")
	}
	if len(record.Summary) > channelValidationSummaryMax {
		record.Summary = record.Summary[:channelValidationSummaryMax]
	}
	if record.CreatedTime == 0 {
		record.CreatedTime = common.GetTimestamp()
	}
	return DB.Create(record).Error
}

// ChannelValidationRecordFilter is the query shape exposed by the
// list endpoint. UserId=0 means "all users" (admin only). Both Verdict
// and Model are optional substring/exact filters.
type ChannelValidationRecordFilter struct {
	UserId   int
	Verdict  string
	Model    string
	Page     int
	PageSize int
}

const (
	channelValidationDefaultPageSize = 20
	channelValidationMaxPageSize     = 100
)

// ListChannelValidationRecords returns paginated records ordered by
// created_time DESC. ResultJson is omitted from the rows to keep the
// response cheap; clients fetch a record by id to read it.
func ListChannelValidationRecords(filter ChannelValidationRecordFilter) ([]ChannelValidationRecord, int64, error) {
	page := filter.Page
	if page < 0 {
		page = 0
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = channelValidationDefaultPageSize
	}
	if pageSize > channelValidationMaxPageSize {
		pageSize = channelValidationMaxPageSize
	}

	query := DB.Model(&ChannelValidationRecord{})
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.Verdict != "" {
		query = query.Where("verdict = ?", filter.Verdict)
	}
	if filter.Model != "" {
		query = query.Where("model = ?", filter.Model)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []ChannelValidationRecord
	err := query.
		Omit("result_json").
		Order("created_time DESC").
		Limit(pageSize).
		Offset(page * pageSize).
		Find(&records).Error
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// GetChannelValidationRecord fetches a single record including
// result_json. ownerUserId scopes the lookup to one user; pass 0 to
// allow any owner (admin lookup).
func GetChannelValidationRecord(id int, ownerUserId int) (*ChannelValidationRecord, error) {
	if id <= 0 {
		return nil, errors.New("invalid record id")
	}
	var record ChannelValidationRecord
	query := DB.Where("id = ?", id)
	if ownerUserId > 0 {
		query = query.Where("user_id = ?", ownerUserId)
	}
	if err := query.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// DeleteChannelValidationRecord removes one record. Scoped by
// ownerUserId in the same way as GetChannelValidationRecord; pass 0
// from admin callers to allow any owner.
func DeleteChannelValidationRecord(id int, ownerUserId int) (int64, error) {
	if id <= 0 {
		return 0, errors.New("invalid record id")
	}
	query := DB.Where("id = ?", id)
	if ownerUserId > 0 {
		query = query.Where("user_id = ?", ownerUserId)
	}
	res := query.Delete(&ChannelValidationRecord{})
	return res.RowsAffected, res.Error
}

// CleanupExpiredChannelValidationRecords removes records older than the
// retention window. Plain DELETE works cross-DB.
func CleanupExpiredChannelValidationRecords(retention time.Duration) error {
	if retention <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-retention).Unix()
	return DB.Where("created_time < ?", cutoff).Delete(&ChannelValidationRecord{}).Error
}
