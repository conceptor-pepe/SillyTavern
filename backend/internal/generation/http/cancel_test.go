// cancel_test.go 验证取消请求通过归属与状态更新后才能中断运行中的 Provider。
package generationhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	genapp "ai-chat/backend/internal/generation/app"
	"ai-chat/backend/internal/generation/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// cancelRepo 模拟数据库条件更新，互斥保护运行协程和取消请求的状态。
type cancelRepo struct {
	domain.Repo
	mu     sync.Mutex
	status string
	err    error
}

// Move 按用户、任务和前置状态执行原子更新，失败不得修改状态。
func (r *cancelRepo) Move(_ context.Context, uid, id uint64, from []string, patch domain.Generation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if uid != 7 || id != 8 || !slices.Contains(from, r.status) {
		return domain.ErrNotFound
	}
	if patch.Status == domain.StatusCancelled && r.err != nil {
		return r.err
	}
	r.status = patch.Status
	return nil
}

// waitingProvider 暴露运行上下文，直到请求被真正取消才退出。
type waitingProvider struct{ started chan context.Context }

// Stream 模拟等待上游响应期间的取消，不依赖休眠判断调用顺序。
func (p waitingProvider) Stream(ctx context.Context, _ provider.Request) (provider.Stream, error) {
	p.started <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

// cancelFixture 保存正在运行的任务与同步观察点。
type cancelFixture struct {
	engine *gin.Engine
	ctx    context.Context
	repo   *cancelRepo
}

// startCancelable 启动真实 Runner，并注册可验证退出的清理函数。
func startCancelable(t *testing.T, uid uint64, cause error) cancelFixture {
	t.Helper()
	repo := &cancelRepo{status: domain.StatusPending, err: cause}
	tasks := genapp.New(repo)
	p := waitingProvider{started: make(chan context.Context, 1)}
	runner := genapp.NewRunner(tasks, p, httpMsgs{})
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := runner.Run(ctx, genapp.RunArgs{UserID: 7, GenerationID: 8, ConversationID: 3})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("run error=%v", err)
		}
	}()
	t.Cleanup(func() {
		stop()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("runner did not exit")
		}
	})
	var running context.Context
	select {
	case running = <-p.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	engine := gin.New()
	New(tasks, runner, Deps{}, zap.NewNop()).Routes(engine, func(c *gin.Context) {
		c.Set("user_id", uid)
	})
	return cancelFixture{engine: engine, ctx: running, repo: repo}
}

// TestCancelOwnership 验证越权、状态冲突和数据库失败均不触发 Provider 取消。
func TestCancelOwnership(t *testing.T) {
	for _, tc := range []struct {
		name   string
		uid    uint64
		cause  error
		status int
	}{
		{"other user", 9, nil, http.StatusNotFound},
		{"database error", 7, errors.New("database unavailable"), http.StatusInternalServerError},
		{"state conflict", 7, domain.ErrNotFound, http.StatusNotFound},
		{"owner", 7, nil, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := startCancelable(t, tc.uid, tc.cause)
			rec := httptest.NewRecorder()
			f.engine.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/generations/8", nil))
			if rec.Code != tc.status {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			cancelled := tc.status == http.StatusNoContent
			if (f.ctx.Err() != nil) != cancelled {
				t.Fatalf("provider context error=%v", f.ctx.Err())
			}
			f.repo.mu.Lock()
			defer f.repo.mu.Unlock()
			expected := domain.StatusRunning
			if cancelled {
				expected = domain.StatusCancelled
			}
			if f.repo.status != expected {
				t.Fatalf("task status=%s expected=%s", f.repo.status, expected)
			}
		})
	}
}

// TestCancelTerminal 验证完成任务和重复取消不覆盖终态。
func TestCancelTerminal(t *testing.T) {
	for _, state := range []string{domain.StatusCompleted, domain.StatusCancelled, domain.StatusFailed} {
		t.Run(state, func(t *testing.T) {
			f := startCancelable(t, 7, nil)
			f.repo.mu.Lock()
			f.repo.status = state
			f.repo.mu.Unlock()
			rec := httptest.NewRecorder()
			f.engine.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/generations/8", nil))
			if rec.Code != http.StatusNotFound || f.ctx.Err() != nil {
				t.Fatalf("status=%d provider error=%v", rec.Code, f.ctx.Err())
			}
			f.repo.mu.Lock()
			defer f.repo.mu.Unlock()
			if f.repo.status != state {
				t.Fatalf("terminal status changed to %s", f.repo.status)
			}
		})
	}
}
