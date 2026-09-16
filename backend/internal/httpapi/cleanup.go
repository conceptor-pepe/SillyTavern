// cleanup.go 管理生成任务补偿的启动扫描、周期执行和停止等待。
package httpapi

import (
	"context"
	"time"

	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/logx"
	"go.uber.org/zap"
)

// taskExpiry 限定调度器只能调用生成域提供的过期用例。
type taskExpiry interface {
	Expire(context.Context, int64) (int64, error)
}

// taskCleaner 持有单个清理协程，数据库关闭前必须等待 done。
type taskCleaner struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// startCleaner 启动即扫描遗留任务，后续串行执行，避免同进程重叠清理。
func startCleaner(tasks taskExpiry, logger *zap.Logger, interval time.Duration) *taskCleaner {
	ctx, cancel := context.WithCancel(context.Background())
	cleaner := &taskCleaner{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(cleaner.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			cleanOnce(ctx, tasks, logger)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return cleaner
}

// cleanOnce 给数据库补偿独立短时限，关闭引起的取消不记作运行故障。
func cleanOnce(ctx context.Context, tasks taskExpiry, logger *zap.Logger) {
	if ctx.Err() != nil {
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	count, err := tasks.Expire(runCtx, time.Now().Add(-domain.RunLimit).Unix())
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		logger.Error("generation cleanup failed", zap.Error(logx.SafeError(err)))
		return
	}
	if count > 0 {
		logger.Info("generation cleanup done", zap.Int64("count", count),
			zap.String("status", domain.StatusFailed), zap.String("error_code", "generation_timeout"))
	}
}

// stop 取消数据库操作并等待退出；超时返回错误，调用方不得提前关闭数据库。
func (c *taskCleaner) stop(ctx context.Context) error {
	c.cancel()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
