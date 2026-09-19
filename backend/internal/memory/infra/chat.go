// chat.go 使用会话行锁保护故事记忆的归属与配额。
package infra

import (
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/model"
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func lockChat(tx *gorm.DB, uid, chatID uint64) error {
	var chat model.Conversation
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND mode = ? AND deleted_at IS NULL", chatID, uid, "story").First(&chat).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

// ListChat 不以客户端提供的会话编号直接读取记忆。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @return 记忆与错误
func (r *Repo) ListChat(ctx context.Context, uid, chatID uint64) ([]domain.Entry, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Conversation{}).Where("id = ? AND user_id = ? AND mode = ? AND deleted_at IS NULL", chatID, uid, "story").Count(&count).Error
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	if count != 1 {
		return nil, domain.ErrNotFound
	}
	var rows []model.Memory
	if err := r.db.WithContext(ctx).Where("user_id = ? AND chat_id = ? AND character_id = 0", uid, chatID).Order("id DESC").Limit(201).Find(&rows).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	out := make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		item, err := readEntry(row)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repo) saveChat(ctx context.Context, item domain.Entry) (domain.Entry, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockChat(tx, item.UserID, item.ChatID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		row, err := entryRow(item)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		query := tx.Model(&model.Memory{}).Where("user_id = ? AND chat_id = ? AND character_id = 0", item.UserID, item.ChatID)
		if err := chatQuota(query, item); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if item.ID != 0 {
			return updateEntry(query, row)
		}
		if err := tx.Create(&row).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		item.ID = row.ID
		return nil
	})
	return item, err
}

func chatQuota(query *gorm.DB, item domain.Entry) error {
	var rows []model.Memory
	if err := query.Find(&rows).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return err
	}
	if len(rows) >= 200 && item.ID == 0 {
		return domain.ErrInvalid
	}
	total := 0
	for _, row := range rows {
		if row.ID != item.ID && row.Enabled && row.Pinned {
			total += len(row.Content) + 3
		}
	}
	if item.Enabled && item.Pinned {
		total += len(item.Content) + 3
	}
	if total > 6000 {
		return domain.ErrInvalid
	}
	return nil
}

// DeleteChat 不允许通过另一会话编号删除记忆。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @param id 记忆编号
// @return 删除错误
func (r *Repo) DeleteChat(ctx context.Context, uid, chatID, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockChat(tx, uid, chatID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		out := tx.Where("id = ? AND user_id = ? AND chat_id = ? AND character_id = 0", id, uid, chatID).Delete(&model.Memory{})
		if out.Error != nil {
			return out.Error
		}
		if out.RowsAffected != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
}
