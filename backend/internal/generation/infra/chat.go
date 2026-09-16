// chat.go 在生成域内维护会话删除时的任务取消及会话可见性错误映射。
package infra

import (
	"context"
	"errors"

	chatdomain "ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/model"
	store "ai-chat/backend/internal/repo"
	"gorm.io/gorm"
)

// CancelChat 使用调用方事务批量取消未结束任务，已完成/失败/取消记录保持原样。
func (r *Repo) CancelChat(ctx context.Context, uid, id uint64, finished int64) error {
	return store.DB(ctx, r.db).Model(&model.Generation{}).
		Where("user_id = ? AND conversation_id = ? AND status IN ?", uid, id,
			[]string{domain.StatusPending, domain.StatusRunning}).
		Updates(map[string]any{"status": domain.StatusCancelled, "finished_at": finished}).Error
}

// findRow 只读取本域任务事实；公开查询和启动必须额外通过会话可见性检查。
func (r *Repo) findRow(ctx context.Context, uid, id uint64) (model.Generation, error) {
	var row model.Generation
	err := store.DB(ctx, r.db).Where("user_id = ? AND id = ?", uid, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Generation{}, domain.ErrNotFound
	}
	return row, err
}

// chatError 对外统一隐藏已删除会话关联的任务，不暴露资源是否曾经存在。
func chatError(err error) error {
	if errors.Is(err, chatdomain.ErrNotFound) {
		return domain.ErrNotFound
	}
	return err
}
