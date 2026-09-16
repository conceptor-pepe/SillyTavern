// watch.go 通过生成域查询同步其他实例的终态，停止仍在消费的模型请求。
package app

import (
	"context"
	"errors"
	"time"

	"ai-chat/backend/internal/generation/domain"
)

// errTaskStopped 表示任务已不再运行，原有数据库终态必须保留。
var errTaskStopped = errors.New("generation no longer running")

// watchTask 为每个运行任务建立可等待的状态监听，完成或失败收尾前必须退出。
func (r *Runner) watchTask(ctx context.Context, args RunArgs, cancel context.CancelCauseFunc) func() {
	watchCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				if err := r.checkTask(watchCtx, args); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()
	return func() {
		stop()
		<-done
	}
}

// checkTask 不跨域读取表，不能确认仍在运行时停止继续付费生成，错误由执行边界记录。
func (r *Runner) checkTask(ctx context.Context, args RunArgs) error {
	queryCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	task, err := r.tasks.Find(queryCtx, args.UserID, args.GenerationID)
	// 正常完成时停止监听也会取消查询，不能因此撤销随后要提交的完整回复。
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return err
	}
	if task.Status != domain.StatusRunning {
		return errTaskStopped
	}
	return nil
}
