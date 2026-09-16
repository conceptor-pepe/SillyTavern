// gate.go 提供会话行锁事务，串行化删除与依赖可见会话的写入。
package infra

import (
	"context"
	"errors"

	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/repo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Gate 只维护会话可见性和行锁，不写入其他业务域的数据。
type Gate struct{ db *gorm.DB }

// NewGate 创建会话事务能力，供装配层注入依赖会话的业务域。
func NewGate(db *gorm.DB) *Gate { return &Gate{db: db} }

// WithChat 锁定当前用户的未删除会话，回调内任何失败都回滚依赖写入。
func (g *Gate) WithChat(ctx context.Context, uid, id uint64, run func(context.Context) error) error {
	return repo.InTx(ctx, g.db, func(ctx context.Context) error {
		var row model.Conversation
		err := repo.DB(ctx, g.db).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		return run(ctx)
	})
}
