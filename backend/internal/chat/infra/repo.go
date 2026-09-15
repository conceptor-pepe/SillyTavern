// repo.go 使用 GORM 实现会话查询。
package infra

import (
	"context"
	"errors"

	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/model"
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
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if err := query.Model(&model.Conversation{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset((page - 1) * size).Limit(size).Order("last_msg_at DESC, id DESC").Find(&rows).Error
	return mapChats(rows), total, err
}

// Recent 查询最近有消息的会话，按最后消息时间倒序返回。
func (r *Repo) Recent(ctx context.Context, userID uint64, page, size int) ([]domain.Conversation, int64, error) {
	var rows []model.Conversation
	var total int64
	query := r.db.WithContext(ctx).Where("user_id = ? AND last_msg_at IS NOT NULL", userID)
	if err := query.Model(&model.Conversation{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset((page - 1) * size).Limit(size).Order("last_msg_at DESC, id DESC").Find(&rows).Error
	return mapChats(rows), total, err
}

// Find 查询指定用户的单个会话。
func (r *Repo) Find(ctx context.Context, userID, id uint64) (domain.Conversation, error) {
	var row model.Conversation
	err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Conversation{}, domain.ErrNotFound
	}
	return toChat(row), err
}

// Owns 判断会话是否属于指定用户。
func (r *Repo) Owns(ctx context.Context, userID, id uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Conversation{}).
		Where("user_id = ? AND id = ?", userID, id).Count(&count).Error
	return count == 1, err
}

// Create 持久化一条用户会话。
func (r *Repo) Create(ctx context.Context, item domain.Conversation) (domain.Conversation, error) {
	row := model.Conversation{
		UserID: item.UserID, CharacterID: item.CharacterID, Title: item.Title,
		Status: item.Status, ExtraData: "{}",
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Conversation{}, err
	}
	return toChat(row), nil
}

// UpdateTitle 按用户范围更新会话标题。
func (r *Repo) UpdateTitle(ctx context.Context, userID, id uint64, title string) error {
	result := r.db.WithContext(ctx).Model(&model.Conversation{}).
		Where("user_id = ? AND id = ?", userID, id).Update("title", title)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Set 设置会话收藏关系，删除操作可重复执行。
func (r *Repo) Set(ctx context.Context, userID, chatID uint64, on bool) error {
	var chat model.Conversation
	if err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, chatID).First(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		return err
	}
	query := r.db.WithContext(ctx).Where("user_id = ? AND character_id = ? AND kind = ?", userID, chatID, "chat")
	if !on {
		return query.Delete(&model.Favorite{}).Error
	}
	return r.db.WithContext(ctx).Where("user_id = ? AND character_id = ? AND kind = ?", userID, chatID, "chat").
		FirstOrCreate(&model.Favorite{UserID: userID, CharacterID: chatID, Kind: "chat"}).Error
}

// Delete 软删除当前用户的会话，避免影响其他用户同编号查询。
func (r *Repo) Delete(ctx context.Context, userID, id uint64) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).
		Delete(&model.Conversation{}).Error
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
	return domain.Conversation{ID: row.ID, UserID: row.UserID, CharacterID: row.CharacterID, Title: row.Title, Status: row.Status, LastMsgAt: row.LastMsgAt}
}
