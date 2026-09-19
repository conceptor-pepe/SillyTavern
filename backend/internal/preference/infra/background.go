// background.go 持久化账号背景，唯一索引保证首次并发保存不产生重复配置。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/preference/domain"
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 保存配置存储依赖。
type Repo struct{ db *gorm.DB }

// New 创建背景仓储。
func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// Get 新账号返回默认纯色背景。
func (r *Repo) Get(ctx context.Context, uid uint64) (domain.Background, error) {
	var row model.Background
	err := r.db.WithContext(ctx).Where("user_id = ?", uid).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Background{}, nil
	}
	return domain.Background{Image: row.Image, Enabled: row.Enabled}, err
}

// Save 原子替换当前账号配置，支持清空图片与关闭背景。
func (r *Repo) Save(ctx context.Context, uid uint64, item domain.Background) error {
	row := model.Background{UserID: uid, Image: item.Image, Enabled: item.Enabled}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"image", "enabled", "updated_at"})}).Create(&row).Error
}
