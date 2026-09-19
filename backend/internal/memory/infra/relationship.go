// relationship.go 以关系档案作为跨故事记忆的访问边界。
package infra

import (
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/model"
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func lockRelationship(tx *gorm.DB, uid, companionID uint64) error {
	var row model.Relationship
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND companion_id = ? AND deleted_at IS NULL", uid, companionID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

// ListRelationship 读取当前用户与角色共享的跨故事记忆。
// @param ctx 请求上下文
// @param uid 用户编号
// @param companionID 角色编号
// @return 记忆列表与错误
func (r *Repo) ListRelationship(ctx context.Context, uid, companionID uint64) ([]domain.Entry, error) {
	if err := lockRelationship(r.db.WithContext(ctx), uid, companionID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	return r.listScope(ctx, uid, companionID)
}

// SaveRelationship 在关系锁内写入跨故事记忆。
// @param ctx 请求上下文
// @param item 记忆内容
// @return 保存结果与错误
func (r *Repo) SaveRelationship(ctx context.Context, item domain.Entry) (domain.Entry, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockRelationship(tx, item.UserID, item.CharacterID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		row, err := entryRow(item)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		query := tx.Model(&model.Memory{}).Where("user_id = ? AND character_id = ? AND chat_id = 0", item.UserID, item.CharacterID)
		if item.ID != 0 {
			return updateEntry(query, row)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if count >= 200 {
			return domain.ErrInvalid
		}
		if err := tx.Create(&row).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		item.ID = row.ID
		return nil
	})
	return item, err
}

// DeleteRelationship 删除本人关系范围内的指定记忆。
// @param ctx 请求上下文
// @param uid 用户编号
// @param companionID 角色编号
// @param id 记忆编号
// @return 删除错误
func (r *Repo) DeleteRelationship(ctx context.Context, uid, companionID, id uint64) error {
	if err := lockRelationship(r.db.WithContext(ctx), uid, companionID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return err
	}
	out := r.db.WithContext(ctx).Unscoped().Where("id = ? AND user_id = ? AND character_id = ? AND chat_id = 0", id, uid, companionID).Delete(&model.Memory{})
	if out.Error != nil {
		return out.Error
	}
	if out.RowsAffected != 1 {
		return domain.ErrNotFound
	}
	return nil
}
