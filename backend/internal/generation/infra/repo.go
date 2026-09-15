// repo.go 使用 GORM 持久化生成任务。
package infra

import (
	"context"
	"errors"
	"time"

	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// Repo 是生成任务 Repository 的 GORM 实现。
type Repo struct{ db *gorm.DB }

// NewRepo 创建生成任务 Repository。
func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// Create 保存新的生成任务。
func (r *Repo) Create(ctx context.Context, item domain.Generation) (domain.Generation, error) {
	row := toRow(item)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Generation{}, err
	}
	return toDomain(row), nil
}

// Find 按用户范围读取生成任务。
func (r *Repo) Find(ctx context.Context, uid, id uint64) (domain.Generation, error) {
	var row model.Generation
	err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", uid, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Generation{}, domain.ErrNotFound
	}
	return toDomain(row), err
}

// Update 按用户范围更新生成状态。
func (r *Repo) Update(ctx context.Context, uid, id uint64, patch domain.Generation) error {
	return r.move(ctx, uid, id, nil, patch)
}

// Move 按用户和前置状态条件更新任务。
func (r *Repo) Move(ctx context.Context, uid, id uint64, from []string, patch domain.Generation) error {
	return r.move(ctx, uid, id, from, patch)
}

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
	query := r.db.WithContext(ctx).Model(&model.Generation{}).Where("user_id = ? AND id = ?", uid, id)
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

// Expire 将超时运行任务统一标记为失败。
func (r *Repo) Expire(ctx context.Context, before int64) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Generation{}).
		Where("status = ? AND started_at IS NOT NULL AND started_at < ?", domain.StatusRunning, before).
		Updates(map[string]any{
			"status": domain.StatusFailed, "error_code": "generation_timeout",
			"error_message": "generation timed out", "finished_at": time.Now().Unix(),
		})
	return result.RowsAffected, result.Error
}

func toRow(item domain.Generation) model.Generation {
	return model.Generation{
		UserID: item.UserID, ConversationID: item.ConversationID, MessageID: item.MessageID,
		ProviderTaskID: item.ProviderTaskID, Provider: item.Provider, Model: item.Model,
		Status: item.Status, ErrorCode: item.ErrorCode, ErrorMessage: item.ErrorMessage,
		StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
	}
}

func toDomain(row model.Generation) domain.Generation {
	return domain.Generation{
		ID: row.ID, UserID: row.UserID, ConversationID: row.ConversationID, MessageID: row.MessageID,
		ProviderTaskID: row.ProviderTaskID, Provider: row.Provider, Model: row.Model, Status: row.Status,
		ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
	}
}
