// write.go 串行化作品修订，软删除不清除会话引用的版本。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/story/domain"
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Save 创建草稿或按预期修订替换整个聚合。
// @param ctx 请求上下文
// @param item 草稿内容
// @return 草稿与错误
func (r *Repo) Save(ctx context.Context, item domain.Story) (domain.Story, error) {
	raw, err := json.Marshal(item.Definition)
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return item, err
	}
	row := model.Story{UserID: item.UserID, Revision: 1, Definition: string(raw)}
	if item.ID == 0 {
		err = r.db.WithContext(ctx).Create(&row).Error
		item.ID, item.Revision = row.ID, row.Revision
		return item, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old, err := lockStory(tx, item.UserID, item.ID)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if old.Revision != item.Revision {
			return domain.ErrConflict
		}
		item.Revision++
		return tx.Model(&old).Updates(map[string]any{"definition": string(raw), "revision": item.Revision}).Error
	})
	return item, err
}

func lockStory(tx *gorm.DB, uid, id uint64) (model.Story, error) {
	var row model.Story
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).First(&row).Error
	return row, mapError(err)
}

// Delete 与冻结及开聊共享作品行锁，防止删除后新建会话。
// @param ctx 请求上下文
// @param uid 作者编号
// @param id 作品编号
// @return 删除错误
func (r *Repo) Delete(ctx context.Context, uid, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := lockStory(tx, uid, id)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		return tx.Model(&row).Update("deleted_at", time.Now().UTC()).Error
	})
}
