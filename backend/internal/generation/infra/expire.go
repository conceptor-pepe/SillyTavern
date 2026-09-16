// expire.go 负责未结束生成任务的超时补偿，保留已有终态。
package infra

import (
	"context"
	"time"

	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/model"
)

// Expire 将截止时间之前的等待或运行任务原子标记为失败。
// @param ctx 清理上下文；before 为 UTC Unix 秒，等于截止时间的任务不处理。
// @return 本次变更数量和存储错误；调用边界负责记录 Zap 日志。
func (r *Repo) Expire(ctx context.Context, before int64) (int64, error) {
	cutoff := time.Unix(before, 0).UTC()
	// pending 尚未启动；异常 running 缺少启动时间时使用创建时间兜底。
	// 状态与时间在同一次 UPDATE 中判断，避免覆盖并发完成或取消的终态。
	result := r.db.WithContext(ctx).Model(&model.Generation{}).
		Where(`(status = ? AND created_at < ?) OR
			(status = ? AND ((started_at IS NOT NULL AND started_at < ?) OR
			(started_at IS NULL AND created_at < ?)))`,
			domain.StatusPending, cutoff, domain.StatusRunning, before, cutoff).
		Updates(map[string]any{
			"status": domain.StatusFailed, "error_code": "generation_timeout",
			"error_message": "generation timed out", "finished_at": time.Now().Unix(),
		})
	return result.RowsAffected, result.Error
}
