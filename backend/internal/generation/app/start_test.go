// start_test.go 验证任务开始前断连时仍使用可用上下文保存失败终态。
package app

import (
	"context"
	"errors"
	"testing"

	"ai-chat/backend/internal/generation/domain"
)

// startRepo 模拟存储拒绝已取消的开始请求，并校验收尾使用新上下文。
type startRepo struct{ taskRepo }

// Move 保留任务状态约束，不允许失败收尾覆盖已有终态。
func (r *startRepo) Move(ctx context.Context, _, _ uint64, from []string, patch domain.Generation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, status := range from {
		if status == r.status {
			r.status = patch.Status
			return nil
		}
	}
	return domain.ErrNotFound
}

// TestStartCancelled 验证开始阶段失败不会留下永久 pending 或覆盖 cancelled。
func TestStartCancelled(t *testing.T) {
	for _, initial := range []string{domain.StatusPending, domain.StatusCancelled} {
		t.Run(initial, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			tasks := &startRepo{taskRepo{status: initial}}
			runner := NewRunner(New(tasks), fakeProvider{}, &msgRepo{})
			_, err := runner.Run(ctx, RunArgs{UserID: 1, ConversationID: 2, GenerationID: 3})
			expected := domain.StatusFailed
			if initial == domain.StatusCancelled {
				expected = initial
			}
			if !errors.Is(err, context.Canceled) || tasks.status != expected {
				t.Fatalf("status=%s expected=%s err=%v", tasks.status, expected, err)
			}
		})
	}
}
