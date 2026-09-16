// remove.go 编排会话删除及依赖任务取消，确保失败时整体回滚。
package app

import (
	"context"
	"errors"

	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/port"
)

// Deleter 表达会话删除能力，HTTP 写用例必须注入完整生命周期实现。
type Deleter interface {
	Delete(ctx context.Context, userID, chatID uint64) error
}

// Remover 通过会话锁协调本域删除与生成域取消，不访问其他域私有表。
type Remover struct {
	chats Deleter
	gate  port.ChatGate
	tasks port.ChatTasks
}

// NewRemover 创建完整删除流程，不提供仅软删除而遗漏依赖任务的降级路径。
func NewRemover(chats Deleter, gate port.ChatGate, tasks port.ChatTasks) *Remover {
	return &Remover{chats: chats, gate: gate, tasks: tasks}
}

// Delete 保持重复及越权删除无副作用；任务取消失败时会话和收藏也必须回滚。
func (r *Remover) Delete(ctx context.Context, uid, id uint64) error {
	entered := false
	err := r.gate.WithChat(ctx, uid, id, func(ctx context.Context) error {
		entered = true
		if err := r.chats.Delete(ctx, uid, id); err != nil {
			return err
		}
		return r.tasks.CancelChat(ctx, uid, id)
	})
	if !entered && errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	return err
}
