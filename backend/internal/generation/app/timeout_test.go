// timeout_test.go 验证生成执行总时限、网络等待取消和安全失败状态。
package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ai-chat/backend/internal/generation/domain"
	provider "ai-chat/backend/internal/provider/domain"
)

// timeoutRepo 保留状态约束并捕获收尾数据，验证过期上下文未用于写库。
type timeoutRepo struct {
	startRepo
	patch domain.Generation
}

// Move 复用带前置状态的测试仓储，拒绝无效上下文和终态覆盖。
func (r *timeoutRepo) Move(ctx context.Context, uid, id uint64, from []string, patch domain.Generation) error {
	if err := r.startRepo.Move(ctx, uid, id, from, patch); err != nil {
		return err
	}
	r.patch = patch
	return nil
}

// deadlineProvider 模拟收到响应头之前的等待，并保存实际收到的截止时间。
type deadlineProvider struct {
	stream   provider.Stream
	deadline time.Time
}

// Stream 无响应流时阻塞至取消，否则交给流读取阶段消费时限。
func (p *deadlineProvider) Stream(ctx context.Context, _ provider.Request) (provider.Stream, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("provider context has no deadline")
	}
	p.deadline = deadline
	if p.stream != nil {
		return p.stream, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestRunDeadline 验证请求头和流读取阶段共用总时限，超时不保存半条消息。
func TestRunDeadline(t *testing.T) {
	for _, stage := range []string{"headers", "body"} {
		t.Run(stage, func(t *testing.T) {
			tasks := &timeoutRepo{startRepo: startRepo{taskRepo{status: domain.StatusPending}}}
			upstream := &deadlineProvider{}
			stream := &waitStream{closed: make(chan struct{})}
			if stage == "body" {
				upstream.stream = stream
			}
			writer := &doneRepo{}
			runner := NewRunner(New(tasks), upstream, writer)
			runner.limit = 25 * time.Millisecond
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			_, err := runner.Run(ctx, RunArgs{UserID: 1, ConversationID: 2, GenerationID: 9})
			if !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				t.Fatalf("own deadline not enforced: run=%v parent=%v", err, ctx.Err())
			}
			if tasks.patch.Status != domain.StatusFailed || tasks.patch.ErrorCode != "generation_timeout" ||
				tasks.patch.ErrorMessage != "generation timed out" || tasks.patch.FinishedAt == nil {
				t.Fatalf("missing timeout state: %+v", tasks.patch)
			}
			if writer.calls != 0 || len(runner.stops) != 0 {
				t.Fatalf("late write or leaked cancel: writes=%d stops=%d", writer.calls, len(runner.stops))
			}
			if stage == "body" {
				checkClosed(t, stream.closed)
			}
		})
	}
}

// checkClosed 检查流关闭已经发生，不能用最终任务失败掩盖连接泄漏。
func checkClosed(t *testing.T, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	default:
		t.Fatal("provider stream was not closed")
	}
}

// TestRunnerLimit 验证默认时限与清理规则一致，成功生成无需等待定时器。
func TestRunnerLimit(t *testing.T) {
	upstream := &deadlineProvider{stream: &fakeStream{events: []provider.Event{
		{Type: "delta", Text: "ok"}, {Type: "done"},
	}}}
	runner := NewRunner(New(&taskRepo{}), upstream, &doneRepo{})
	started := time.Now()
	if _, err := runner.Run(t.Context(), RunArgs{UserID: 1, ConversationID: 2, GenerationID: 3}); err != nil {
		t.Fatal(err)
	}
	limit := upstream.deadline.Sub(started)
	if limit < domain.RunLimit || limit > domain.RunLimit+time.Second {
		t.Fatalf("provider deadline differs from cleanup policy: %v", limit)
	}
}

// TestFailurePrivacy 验证可查询的任务错误不包含供应商或数据库原始隐私文本。
func TestFailurePrivacy(t *testing.T) {
	tasks := &timeoutRepo{startRepo: startRepo{taskRepo{status: domain.StatusRunning}}}
	runner := NewRunner(New(tasks), nil, nil)
	cause := errors.New("private-provider-key-and-prompt")
	_, err := runner.fail(t.Context(), RunArgs{UserID: 1, GenerationID: 9}, cause)
	if !errors.Is(err, cause) || tasks.patch.ErrorMessage != "generation failed" {
		t.Fatalf("failure classification: patch=%+v err=%v", tasks.patch, err)
	}
}
