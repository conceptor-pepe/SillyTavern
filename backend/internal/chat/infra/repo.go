// repo.go 使用 GORM 实现会话查询。
package infra

import (
	"context"
	"errors"

	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/model"
	store "ai-chat/backend/internal/repo"
	"gorm.io/gorm"
)

// Repo 是会话查询的 GORM 实现。
type Repo struct{ db *gorm.DB }

// NewRepo 创建会话 Repository。
func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// List 查询指定用户的会话列表。
func (r *Repo) List(ctx context.Context, userID uint64, page, size int) ([]domain.Conversation, int64, error) {
	var rows []model.Conversation
	var total int64
	query := r.visible(ctx, userID)
	if err := query.Model(&model.Conversation{}).Count(&total).Error; err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return nil, 0, err
	}
	err := query.Offset((page - 1) * size).Limit(size).Order("last_msg_at DESC, id DESC").Find(&rows).Error
	return mapChats(rows), total, err
}

// Recent 查询最近有消息的会话，按最后消息时间倒序返回。
func (r *Repo) Recent(ctx context.Context, userID uint64, page, size int) ([]domain.Conversation, int64, error) {
	var rows []model.Conversation
	var total int64
	query := r.visible(ctx, userID).Where("last_msg_at IS NOT NULL")
	if err := query.Model(&model.Conversation{}).Count(&total).Error; err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return nil, 0, err
	}
	err := query.Offset((page - 1) * size).Limit(size).Order("last_msg_at DESC, id DESC").Find(&rows).Error
	return mapChats(rows), total, err
}

// Find 查询指定用户的单个会话。
func (r *Repo) Find(ctx context.Context, userID, id uint64) (domain.Conversation, error) {
	var row model.Conversation
	err := r.visible(ctx, userID).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Conversation{}, domain.ErrNotFound
	}
	return toChat(row), err
}

// Owns 判断会话是否属于指定用户。
func (r *Repo) Owns(ctx context.Context, userID, id uint64) (bool, error) {
	var count int64
	err := r.visible(ctx, userID).Where("id = ?", id).Count(&count).Error
	return count == 1, err
}

// Create 持久化一条用户会话。
func (r *Repo) Create(ctx context.Context, item domain.Conversation) (domain.Conversation, error) {
	row := model.Conversation{
		UserID: item.UserID, CharacterID: item.CharacterID, Title: item.Title,
		Status: item.Status, ExtraData: "{}",
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		relation := model.Relationship{UserID: item.UserID, CompanionID: item.CharacterID,
			Stage: "acquaintance", Narrative: "", Milestones: "[]", Revision: 1}
		return tx.Where("user_id = ? AND companion_id = ?", item.UserID, item.CharacterID).FirstOrCreate(&relation).Error
	})
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return domain.Conversation{}, err
	}
	return toChat(row), nil
}

// UpdateTitle 按用户范围更新会话标题。
func (r *Repo) UpdateTitle(ctx context.Context, userID, id uint64, title string) error {
	result := r.visible(ctx, userID).Where("id = ?", id).Update("title", title)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// visible 统一用户归属和软删除条件，Base 的时间指针不会自动添加 GORM 删除作用域。
func (r *Repo) visible(ctx context.Context, userID uint64) *gorm.DB {
	return store.DB(ctx, r.db).Model(&model.Conversation{}).
		Where("user_id = ? AND deleted_at IS NULL", userID)
}

// mapChats 转换会话列表。
func mapChats(rows []model.Conversation) []domain.Conversation {
	out := make([]domain.Conversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toChat(row))
	}
	return out
}

// toChat 转换单个会话模型。
func toChat(row model.Conversation) domain.Conversation {
	return domain.Conversation{Mode: row.Mode, ID: row.ID, UserID: row.UserID, CharacterID: row.CharacterID, Title: row.Title, Status: row.Status, LastMsgAt: row.LastMsgAt}
}
