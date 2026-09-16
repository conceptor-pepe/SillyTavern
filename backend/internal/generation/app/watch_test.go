// watch_test.go 验证跨实例状态监听的归属、终态判断和协程停止。
package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ai-chat/backend/internal/generation/domain"
)

// watchRepo 只替换状态读取，避免测试依赖内存取消表的实现。
type watchRepo struct {
	taskRepo
	find func(context.Context, uint64, uint64) (domain.Generation, error)
}

// Find 透传查询上下文和归属参数，由各场景检查是否正确。
func (r *watchRepo) Find(ctx context.Context, uid, id uint64) (domain.Generation, error) {
	return r.find(ctx, uid, id)
}

// TestCheckTask 验证仅 running 允许继续，查询失败和所有非运行状态都应终止上游。
func TestCheckTask(t *testing.T) {
	dbErr := errors.New("database unavailable")
	for _, scenario := range []struct {
		status string
		cause  error
		want   error
	}{
		{domain.StatusRunning, nil, nil},
		{domain.StatusPending, nil, errTaskStopped},
		{domain.StatusCompleted, nil, errTaskStopped},
		{domain.StatusFailed, nil, errTaskStopped},
		{domain.StatusCancelled, nil, errTaskStopped},
		{"", domain.ErrNotFound, domain.ErrNotFound},
		{"", dbErr, dbErr},
	} {
		repo := &watchRepo{find: func(ctx context.Context, uid, id uint64) (domain.Generation, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 2*time.Second || uid != 7 || id != 9 {
				t.Errorf("invalid query scope or deadline: uid=%d id=%d deadline=%v", uid, id, deadline)
			}
			return domain.Generation{Status: scenario.status}, scenario.cause
		}}
		runner := NewRunner(New(repo), nil, nil)
		err := runner.checkTask(t.Context(), RunArgs{UserID: 7, GenerationID: 9})
		if !errors.Is(err, scenario.want) {
			t.Fatalf("status=%s error=%v want=%v", scenario.status, err, scenario.want)
		}
	}
}

// TestWatchStop 验证主动停止监听会打断在途查询，但不会取消仍需保存结果的运行上下文。
func TestWatchStop(t *testing.T) {
	started := make(chan struct{})
	repo := &watchRepo{find: func(ctx context.Context, _, _ uint64) (domain.Generation, error) {
		close(started)
		<-ctx.Done()
		return domain.Generation{}, ctx.Err()
	}}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	runner := NewRunner(New(repo), nil, nil)
	stop := runner.watchTask(ctx, RunArgs{UserID: 7, GenerationID: 9}, cancel)
	defer stop()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("watch did not read task")
	}
	stop()
	if ctx.Err() != nil {
		t.Fatalf("normal watcher stop cancelled result persistence: %v", context.Cause(ctx))
	}
}

// TestWatchCancel 验证监听发现远端终态后取消运行上下文，并保留明确的终止原因。
func TestWatchCancel(t *testing.T) {
	repo := &watchRepo{find: func(context.Context, uint64, uint64) (domain.Generation, error) {
		return domain.Generation{Status: domain.StatusCancelled}, nil
	}}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	runner := NewRunner(New(repo), nil, nil)
	stop := runner.watchTask(ctx, RunArgs{UserID: 7, GenerationID: 9}, cancel)
	defer stop()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("remote cancellation was not observed")
	}
	if !errors.Is(context.Cause(ctx), errTaskStopped) {
		t.Fatalf("lost stop cause: %v", context.Cause(ctx))
	}
}

// TestWatchFailure 验证状态查询故障会停止生成，并将错误原因传到执行边界。
func TestWatchFailure(t *testing.T) {
	cause := errors.New("database unavailable")
	repo := &watchRepo{find: func(context.Context, uint64, uint64) (domain.Generation, error) {
		return domain.Generation{}, cause
	}}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	runner := NewRunner(New(repo), nil, nil)
	stop := runner.watchTask(ctx, RunArgs{UserID: 7, GenerationID: 9}, cancel)
	defer stop()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("failed state query did not stop generation")
	}
	if !errors.Is(context.Cause(ctx), cause) {
		t.Fatalf("lost database failure: %v", context.Cause(ctx))
	}
}
