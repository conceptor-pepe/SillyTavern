// repo.go 使用 GORM 持久化生成任务。
package infra

import (
	"context"

	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/port"
	store "ai-chat/backend/internal/repo"
	"gorm.io/gorm"
)

// Repo 是生成任务 Repository 的 GORM 实现。
type Repo struct {
	db    *gorm.DB
	chats port.ChatGate
}

// NewRepo 创建生成任务 Repository。
func NewRepo(db *gorm.DB, chats port.ChatGate) *Repo { return &Repo{db: db, chats: chats} }

// Create 保存新的生成任务。
func (r *Repo) Create(ctx context.Context, item domain.Generation) (domain.Generation, error) {
	row := toRow(item)
	err := r.chats.WithChat(ctx, item.UserID, item.ConversationID, func(ctx context.Context) error {
		return store.DB(ctx, r.db).Create(&row).Error
	})
	if err != nil {
		return domain.Generation{}, err
	}
	return toDomain(row), nil
}

// Find 按用户范围读取生成任务。
func (r *Repo) Find(ctx context.Context, uid, id uint64) (domain.Generation, error) {
	row, err := r.findRow(ctx, uid, id)
	if err != nil {
		return domain.Generation{}, err
	}
	err = r.chats.WithChat(ctx, uid, row.ConversationID, func(ctx context.Context) error {
		var err error
		row, err = r.findRow(ctx, uid, id)
		return err
	})
	if err != nil {
		return domain.Generation{}, chatError(err)
	}
	return toDomain(row), nil
}

// Update 按用户范围更新生成状态。
func (r *Repo) Update(ctx context.Context, uid, id uint64, patch domain.Generation) error {
	return r.Move(ctx, uid, id, nil, patch)
}

// Move 按用户和前置状态条件更新任务。
func (r *Repo) Move(ctx context.Context, uid, id uint64, from []string, patch domain.Generation) error {
	if patch.Status != domain.StatusRunning && patch.Status != domain.StatusCompleted {
		return r.move(ctx, uid, id, from, patch)
	}
	row, err := r.findRow(ctx, uid, id)
	if err != nil {
		return err
	}
	err = r.chats.WithChat(ctx, uid, row.ConversationID, func(ctx context.Context) error {
		return r.move(ctx, uid, id, from, patch)
	})
	return chatError(err)
}

// move 以用户和前置状态约束更新，避免迟到收尾覆盖已有终态。
func (r *Repo) move(ctx context.Context, uid, id uint64, from []string, patch domain.Generation) error {
	updates := map[string]any{"status": patch.Status}
	if patch.MessageID != nil {
		updates["message_id"] = patch.MessageID
	}
	if patch.StartedAt != nil {
		updates["started_at"] = patch.StartedAt
	}
	if patch.FinishedAt != nil {
		updates["finished_at"] = patch.FinishedAt
	}
	if patch.ErrorCode != "" {
		updates["error_code"] = patch.ErrorCode
		updates["error_message"] = patch.ErrorMessage
	}
	query := store.DB(ctx, r.db).Model(&model.Generation{}).Where("user_id = ? AND id = ?", uid, id)
	if len(from) > 0 {
		query = query.Where("status IN ?", from)
	}
	result := query.Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// toRow 将领域任务映射到存储字段，不接受调用方指定数据库主键。
func toRow(item domain.Generation) model.Generation {
	return model.Generation{
		UserID: item.UserID, ConversationID: item.ConversationID, MessageID: item.MessageID,
		ProviderTaskID: item.ProviderTaskID, Provider: item.Provider, Model: item.Model,
		Status: item.Status, ErrorCode: item.ErrorCode, ErrorMessage: item.ErrorMessage,
		StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
	}
}

// toDomain 返回任务事实，保留来源归属和可空生命周期时间。
func toDomain(row model.Generation) domain.Generation {
	return domain.Generation{
		ID: row.ID, UserID: row.UserID, ConversationID: row.ConversationID, MessageID: row.MessageID,
		ProviderTaskID: row.ProviderTaskID, Provider: row.Provider, Model: row.Model, Status: row.Status,
		ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
	}
}
