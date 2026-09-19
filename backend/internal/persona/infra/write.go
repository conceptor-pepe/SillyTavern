// write.go 通过行锁与修订号保护人设更新。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/persona/domain"
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Save 创建或替换本人预期修订的人设。
// @param ctx 请求上下文
// @param p 玩家人设
// @return 人设与错误
func (r *Repo) Save(ctx context.Context, p domain.Persona) (domain.Persona, error) {
	row := model.PlayerPersona{UserID: p.UserID, Revision: 1, Name: p.Name, Description: p.Description, Avatar: p.Avatar}
	if p.ID == 0 {
		err := r.db.WithContext(ctx).Create(&row).Error
		return convert(row), err
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old, err := lockPersona(tx, p.UserID, p.ID)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if old.Revision != p.Revision {
			return domain.ErrConflict
		}
		row.ID, row.Revision = old.ID, old.Revision+1
		return tx.Model(&old).Select("name", "description", "avatar", "revision").Updates(row).Error
	})
	return convert(row), err
}

func lockPersona(tx *gorm.DB, uid, id uint64) (model.PlayerPersona, error) {
	var row model.PlayerPersona
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).First(&row).Error
	return row, mapError(err)
}

// Delete 与开聊复制快照串行，已有会话不依赖原记录。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 人设编号
// @return 删除错误
func (r *Repo) Delete(ctx context.Context, uid, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockPersona(tx, uid, id)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		return tx.Model(&row).Update("deleted_at", time.Now().UTC()).Error
	})
}
