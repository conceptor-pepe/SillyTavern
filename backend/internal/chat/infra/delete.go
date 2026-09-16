// delete.go 在调用方事务内软删除会话并清除本域收藏，不触碰生成域数据。
package infra

import (
	"context"
	"time"

	"ai-chat/backend/internal/model"
	store "ai-chat/backend/internal/repo"
)

// Delete 原子更新会话和收藏；完整依赖取消由应用层 Remover 编排。
func (r *Repo) Delete(ctx context.Context, uid, id uint64) error {
	return store.InTx(ctx, r.db, func(ctx context.Context) error {
		result := r.visible(ctx, uid).Where("id = ?", id).Update("deleted_at", time.Now().UTC())
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return store.DB(ctx, r.db).Where("user_id = ? AND character_id = ? AND kind = ?", uid, id, "chat").
			Delete(&model.Favorite{}).Error
	})
}
