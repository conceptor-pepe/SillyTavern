// favorite.go 将会话收藏写入放入会话行锁，防止删除后重建孤立收藏。
package infra

import (
	"context"

	"ai-chat/backend/internal/model"
	store "ai-chat/backend/internal/repo"
)

// Set 与删除串行化；重复收藏和取消收藏保持幂等。
func (r *Repo) Set(ctx context.Context, uid, id uint64, on bool) error {
	return NewGate(r.db).WithChat(ctx, uid, id, func(ctx context.Context) error {
		query := store.DB(ctx, r.db).Where("user_id = ? AND character_id = ? AND kind = ?", uid, id, "chat")
		if !on {
			return query.Delete(&model.Favorite{}).Error
		}
		return query.FirstOrCreate(&model.Favorite{UserID: uid, CharacterID: id, Kind: "chat"}).Error
	})
}
