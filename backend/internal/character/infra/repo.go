// repo.go 使用 GORM 实现角色域查询。
package infra

import (
	"context"
	"encoding/json"

	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// Repo 是角色查询的 GORM 实现。
type Repo struct{ db *gorm.DB }

// NewRepo 创建角色 Repository。
func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// Create 持久化角色并返回领域对象。
func (r *Repo) Create(ctx context.Context, item domain.Character) (domain.Character, error) {
	tags, err := json.Marshal(item.Tags)
	if err != nil {
		// audit:allow-no-log 角色 Handler 统一记录序列化失败和用户上下文。
		return domain.Character{}, err
	}
	extra, err := json.Marshal(profileData{Portrait: item.Portrait, Gender: item.Gender, Age: item.Age})
	if err != nil {
		// audit:allow-no-log 角色 Handler 统一记录序列化失败和用户上下文。
		return domain.Character{}, err
	}
	row := model.Character{
		UserID: item.UserID, Name: item.Name, Description: item.Description,
		Personality: item.Personality, Scenario: item.Scenario, FirstMessage: item.FirstMessage,
		MessageSample: item.MessageSample, Creator: "user", CreatorNotes: "", Tags: string(tags), ExtraData: string(extra),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		// audit:allow-no-log 由角色 Handler 统一记录创建失败和用户上下文。
		return domain.Character{}, err
	}
	return toChar(row), nil
}

// List 查询指定用户可见的角色。
func (r *Repo) List(ctx context.Context, userID uint64, page, size int) ([]domain.Character, int64, error) {
	var rows []model.Character
	var total int64
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if err := query.Model(&model.Character{}).Count(&total).Error; err != nil {
		// audit:allow-no-log 由角色 Handler 统一记录查询失败和用户上下文。
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
	item := domain.Character{ID: row.ID, UserID: row.UserID, Name: row.Name, Description: row.Description, Personality: row.Personality, Scenario: row.Scenario, FirstMessage: row.FirstMessage, MessageSample: row.MessageSample}
	readProfile(row, &item)
	return item
}
