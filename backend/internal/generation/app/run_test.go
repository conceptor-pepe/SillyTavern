// run_test.go 验证生成编排的成功和失败状态流转。
package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ai-chat/backend/internal/generation/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
)

type taskRepo struct {
	item   domain.Generation
	status string
}

func (r *taskRepo) Create(_ context.Context, item domain.Generation) (domain.Generation, error) {
	item.ID = 9
	r.item = item
	return item, nil
}
func (r *taskRepo) Find(context.Context, uint64, uint64) (domain.Generation, error) {
	return r.item, nil
}
func (r *taskRepo) Update(context.Context, uint64, uint64, domain.Generation) error { return nil }
func (r *taskRepo) Move(_ context.Context, _ uint64, _ uint64, _ []string, patch domain.Generation) error {
	r.status = patch.Status
	return nil
}
func (r *taskRepo) Expire(context.Context, int64) (int64, error) { return 0, nil }

// CancelChat 满足批量取消能力，Runner 单元测试不代替删除事务集成测试。
func (r *taskRepo) CancelChat(context.Context, uint64, uint64, int64) error { return nil }

type fakeStream struct {
	events []provider.Event
	index  int
}

func (s *fakeStream) Next(context.Context) (provider.Event, error) {
	if s.index >= len(s.events) {
		return provider.Event{}, errors.New("stream ended")
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}
func (s *fakeStream) Close() error { return nil }

type fakeProvider struct{ stream provider.Stream }

func (p fakeProvider) Stream(context.Context, provider.Request) (provider.Stream, error) {
	return p.stream, nil
}

type msgRepo struct{ item msgdomain.Message }

func (r *msgRepo) Create(_ context.Context, _ uint64, item msgdomain.Message) (msgdomain.Message, error) {
	item.ID = 11
	r.item = item
	return item, nil
}

type doneRepo struct {
	msgdomain.Message
	calls int
	err   error
}

// SaveDone 模拟消息和生成状态的原子落库边界。
func (r *doneRepo) SaveDone(_ context.Context, _, _ uint64, item msgdomain.Message, _ int64) (msgdomain.Message, error) {
	r.calls++
	if r.err != nil {
		return msgdomain.Message{}, r.err
	}
	item.ID = 12
	r.Message = item
	return item, nil
}

// TestCandidateSaveError 验证事务失败不会返回成功消息，且会将生成任务转为失败。
func TestCandidateSaveError(t *testing.T) {
	cause := errors.New("transaction failed")
	writer := &doneRepo{err: cause}
	tasks := &taskRepo{}
	runner := NewRunner(New(tasks), fakeProvider{stream: &fakeStream{
		events: []provider.Event{
			{Type: "delta", Text: "A"}, {Type: "delta", Index: 1, Text: "B"}, {Type: "done"},
		},
	}}, writer)
	item, err := runner.Run(context.Background(), RunArgs{
		UserID: 1, ConversationID: 2, GenerationID: 9, Request: provider.Request{N: 2},
	})
	if !errors.Is(err, cause) || item.ID != 0 || tasks.status != domain.StatusFailed {
		t.Fatalf("item=%+v status=%s err=%v", item, tasks.status, err)
	}
}

// Create 保持 DoneWriter 同时满足消息写入接口。
func (r *doneRepo) Create(_ context.Context, _ uint64, item msgdomain.Message) (msgdomain.Message, error) {
	return item, nil
}

type waitStream struct{ closed chan struct{} }

func (s *waitStream) Next(ctx context.Context) (provider.Event, error) {
	select {
	case <-ctx.Done():
		return provider.Event{}, ctx.Err()
	case <-s.closed:
		return provider.Event{}, errors.New("closed")
	}
}

func (s *waitStream) Close() error { close(s.closed); return nil }

// TestSaveCandidates 验证不同候选独立组装并交由统一事务写入器保存。
func TestSaveCandidates(t *testing.T) {
	tasks := &taskRepo{}
	writer := &doneRepo{}
	runner := NewRunner(New(tasks), fakeProvider{stream: &fakeStream{
		events: []provider.Event{
			{Type: "delta", Index: 0, Text: "first"},
			{Type: "delta", Index: 1, Text: "second"},
			{Type: "done"},
		},
	}}, writer)
	item, err := runner.Run(context.Background(), RunArgs{
		UserID: 1, ConversationID: 2, GenerationID: 9,
		Request: provider.Request{Model: "demo", N: 2},
	})
	if err != nil || item.ID != 12 || writer.calls != 1 {
		t.Fatalf("item=%+v calls=%d err=%v", item, writer.calls, err)
	}
	if item.Content != "first" || len(writer.Variants) != 2 || writer.Variants[1].Content != "second" {
		t.Fatalf("message=%+v", writer.Message)
	}
}

// TestRun 验证文本增量会保存为 assistant 消息。
func TestRun(t *testing.T) {
	tasks := &taskRepo{}
	msgs := &msgRepo{}
	runner := NewRunner(New(tasks), fakeProvider{stream: &fakeStream{
		events: []provider.Event{{Type: "delta", Text: "a"}, {Type: "done"}},
	}}, msgs)
	item, err := runner.Run(context.Background(), RunArgs{
		UserID: 1, ConversationID: 2, GenerationID: 9,
		Request: provider.Request{Model: "demo"},
	})
	if err != nil || item.Content != "a" || tasks.status != domain.StatusCompleted {
		t.Fatalf("item=%+v status=%s err=%v", item, tasks.status, err)
	}
}

// TestRunKeepsParent 验证生成的新消息保留分支父消息且不覆盖旧消息。
func TestRunKeepsParent(t *testing.T) {
	parent := uint64(7)
	writer := &msgRepo{}
	tasks := &taskRepo{}
	runner := NewRunner(New(tasks), fakeProvider{stream: &fakeStream{
		events: []provider.Event{{Type: "delta", Text: "hello"}, {Type: "done"}},
	}}, writer)
	item, err := runner.Run(context.Background(), RunArgs{
		UserID: 1, ConversationID: 2, GenerationID: 3,
		ParentID: &parent, Request: provider.Request{Model: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.ParentID == nil || *item.ParentID != parent {
		t.Fatalf("parent id = %v", item.ParentID)
	}
	if writer.item.ID == 7 {
		t.Fatal("generation overwrote the parent message")
	}
}

// TestRunUsesDoneWriter 验证原子写入路径不会先创建重复 assistant 消息。
func TestRunUsesDoneWriter(t *testing.T) {
	writer := &doneRepo{}
	runner := NewRunner(New(&taskRepo{}), fakeProvider{stream: &fakeStream{
		events: []provider.Event{{Type: "delta", Text: "done"}, {Type: "done"}},
	}}, writer)
	item, err := runner.Run(context.Background(), RunArgs{
		UserID: 1, ConversationID: 2, GenerationID: 3,
		Request: provider.Request{Model: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if writer.calls != 1 || item.ID != 12 || item.Content != "done" {
		t.Fatalf("calls=%d item=%+v", writer.calls, item)
	}
}

// TestCancel 验证取消注册表可以中断正在等待 Provider 的任务。
func TestCancel(t *testing.T) {
	tasks := &taskRepo{}
	stream := &waitStream{closed: make(chan struct{})}
	runner := NewRunner(New(tasks), fakeProvider{stream: stream}, &msgRepo{})
	done := make(chan error, 1)
	go func() {
		_, err := runner.Run(context.Background(), RunArgs{
			UserID: 1, ConversationID: 2, GenerationID: 9,
			Request: provider.Request{Model: "demo"},
		})
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	runner.Cancel(9)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop provider")
	}
}
