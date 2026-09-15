// repo.go 使用 GORM 实现角色域查询。
package infra

import (
	"context"

	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// Repo 是角色查询的 GORM 实现。
type Repo struct{ db *gorm.DB }

// NewRepo 创建角色 Repository。
func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// List 查询指定用户可见的角色。
func (r *Repo) List(ctx context.Context, userID uint64, page, size int) ([]domain.Character, int64, error) {
	var rows []model.Character
	var total int64
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if err := query.Model(&model.Character{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset((page - 1) * size).Limit(size).Order("id DESC").Find(&rows).Error
	return mapChars(rows), total, err
}

// Find 查询指定用户拥有的单个角色。
func (r *Repo) Find(ctx context.Context, userID, id uint64) (domain.Character, error) {
	var row model.Character
	err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&row).Error
	return toChar(row), err
}

// Owns 判断角色是否属于指定用户。
func (r *Repo) Owns(ctx context.Context, userID, id uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Character{}).
		Where("user_id = ? AND id = ?", userID, id).Count(&count).Error
	return count == 1, err
}

// mapChars 转换角色列表，避免向上层暴露 GORM 模型。
func mapChars(rows []model.Character) []domain.Character {
	out := make([]domain.Character, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChar(row))
	}
	return out
}

// toChar 转换单个角色模型。
func toChar(row model.Character) domain.Character {
	return domain.Character{ID: row.ID, UserID: row.UserID, Name: row.Name, Description: row.Description, Personality: row.Personality, Scenario: row.Scenario, FirstMessage: row.FirstMessage}
}
