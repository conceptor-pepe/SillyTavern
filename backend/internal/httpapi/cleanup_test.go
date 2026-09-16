// cleanup_test.go 验证启动扫描、周期执行、日志脱敏和清理停止等待。
package httpapi

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-chat/backend/internal/generation/domain"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// expiryFunc 使调度测试只依赖生成域过期能力，不模拟数据库实现。
type expiryFunc func(context.Context, int64) (int64, error)

// Expire 将调用转交给测试场景，保留上下文和截止时间参数。
func (f expiryFunc) Expire(ctx context.Context, before int64) (int64, error) {
	return f(ctx, before)
}

// TestCleanupLoop 验证启动扫描、后续周期、统一超时标准和有界数据库上下文。
func TestCleanupLoop(t *testing.T) {
	calls := make(chan struct{}, 4)
	tasks := expiryFunc(func(ctx context.Context, before int64) (int64, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 5*time.Second || ctx.Err() != nil {
			t.Error("cleanup has no valid bounded context")
		}
		if age := time.Since(time.Unix(before, 0)); age < domain.RunLimit || age > domain.RunLimit+time.Second {
			t.Errorf("cleanup age=%v", age)
		}
		select {
		case calls <- struct{}{}:
		default:
		}
		return 1, nil
	})
	cleaner := startCleaner(tasks, zap.NewNop(), 10*time.Millisecond)
	defer cleaner.cancel()
	for i := 0; i < 2; i++ {
		select {
		case <-calls:
		case <-time.After(time.Second):
			t.Fatal("missing startup or periodic cleanup")
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := cleaner.stop(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestCleanerStop 验证停止不会把取消信号误当成协程已退出。
func TestCleanerStop(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	tasks := expiryFunc(func(ctx context.Context, _ int64) (int64, error) {
		close(entered)
		<-ctx.Done()
		<-release
		return 0, ctx.Err()
	})
	cleaner := startCleaner(tasks, zap.NewNop(), time.Minute)
	defer cleaner.cancel()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("startup scan did not run immediately")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if err := cleaner.stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop did not wait: %v", err)
	}
	unblock()
	wait, stop := context.WithTimeout(t.Context(), time.Second)
	defer stop()
	if err := cleaner.stop(wait); err != nil {
		t.Fatal(err)
	}
}

// TestCleanupLogs 验证失败脱敏、成功计数以及关闭取消不产生日志噪声。
func TestCleanupLogs(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	secret := "private-sql-credential"
	tasks := expiryFunc(func(context.Context, int64) (int64, error) { return 0, errors.New(secret) })
	cleanOnce(t.Context(), tasks, logger)
	tasks = expiryFunc(func(context.Context, int64) (int64, error) { return 2, nil })
	cleanOnce(t.Context(), tasks, logger)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cleanOnce(ctx, tasks, logger)
	entries := logs.All()
	if len(entries) != 2 || entries[0].Level != zap.ErrorLevel || entries[1].ContextMap()["count"] != int64(2) {
		t.Fatalf("unexpected logs: %+v", entries)
	}
	if strings.Contains(entries[0].ContextMap()["error"].(string), secret) {
		t.Fatal("raw dependency error leaked")
	}
}
