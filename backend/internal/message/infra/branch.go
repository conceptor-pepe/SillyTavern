// branch.go 使用有界递归查询读取一条消息分支，避免逐节点查询和混入兄弟回复。
package infra

import (
	"context"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
)

// branchSQL 在同一查询快照内校验会话归属及祖先可见性，最多读取最近一百条。
const branchSQL = `
WITH RECURSIVE branch AS (
 SELECT m.id, m.parent_id, m.conversation_id, 1 AS depth
 FROM messages m
 JOIN conversations c ON c.id = m.conversation_id
 WHERE m.id = ? AND m.conversation_id = ? AND c.user_id = ?
   AND m.deleted_at IS NULL AND c.deleted_at IS NULL
 UNION ALL
 SELECT m.id, m.parent_id, m.conversation_id, b.depth + 1
 FROM messages m JOIN branch b ON m.id = b.parent_id
 WHERE m.conversation_id = b.conversation_id AND m.deleted_at IS NULL
   AND b.depth < 100
)
SELECT m.* FROM branch b JOIN messages m ON m.id = b.id ORDER BY b.depth DESC`

// Branch 只返回当前用户指定会话中的祖先链，空结果隐藏消息存在性。
func (r *Repo) Branch(ctx context.Context, uid, chatID, messageID uint64) ([]domain.Message, error) {
	var rows []model.Message
	if err := r.db.WithContext(ctx).Raw(branchSQL, messageID, chatID, uid).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, domain.ErrNotFound
	}
	return mapMsgs(rows), nil
}
