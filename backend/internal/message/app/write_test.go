// write_test.go 验证消息编辑和删除用例的输入边界与能力隔离。
package app

import (
	"context"
	"errors"
	"testing"

	"ai-chat/backend/internal/message/domain"
)

// fakeMutator 记录消息变更调用，隔离应用层测试与数据库。
type fakeMutator struct {
	updated bool
	deleted bool
	revised bool
	err     error
}

// Update 模拟消息编辑结果。
func (f *fakeMutator) Update(_ context.Context, _, _ uint64, content string) (domain.Message, error) {
	f.updated = true
	return domain.Message{ID: 2, Content: content}, f.err
}

// Delete 模拟消息软删除。
func (f *fakeMutator) Delete(_ context.Context, _, _ uint64) error {
	f.deleted = true
	return f.err
}

// ReviseAssistant 模拟创建 AI 编辑分支。
func (f *fakeMutator) ReviseAssistant(_ context.Context, _, _ uint64, content string) (domain.Message, error) {
	f.revised = true
	return domain.Message{ID: 3, Role: "assistant", Content: content}, f.err
}

// fakeRepo 同时提供创建能力和消息变更能力。
type fakeRepo struct{ *fakeMutator }

// parentRepo 模拟父消息归属查询结果。
type parentRepo struct {
	fakeRepo
	owned bool
	err   error
}

// fakeChat 模拟当前用户拥有目标会话。
type fakeChat struct{}

// Owns 返回会话归属结果。
func (fakeChat) Owns(context.Context, uint64, uint64) (bool, error) {
	return true, nil
}

// OwnsInChat 返回预设的父消息归属结果。
func (f *parentRepo) OwnsInChat(context.Context, uint64, uint64, uint64) (bool, error) {
	return f.owned, f.err
}

// List 返回空历史，满足消息 Repository 契约。
func (f *fakeRepo) List(context.Context, uint64, uint64, int, int) ([]domain.Message, int64, error) {
	return nil, 0, nil
}

// Create 返回输入消息，满足消息 Repository 契约。
func (f *fakeRepo) Create(_ context.Context, _ uint64, item domain.Message) (domain.Message, error) {
	return item, nil
}

// TestWriteEditDelete 验证合法编辑删除会调用窄变更接口。
func TestWriteEditDelete(t *testing.T) {
	mutator := &fakeMutator{}
	write := NewWrite(&fakeRepo{mutator}, nil)
	item, err := write.Edit(context.Background(), 1, 2, "updated")
	if err != nil || item.Content != "updated" || !mutator.updated {
		t.Fatalf("item=%+v err=%v updated=%v", item, err, mutator.updated)
	}
	if err := write.Delete(context.Background(), 1, 2); err != nil || !mutator.deleted {
		t.Fatalf("err=%v deleted=%v", err, mutator.deleted)
	}
	revised, err := write.ReviseAssistant(context.Background(), 1, 2, "edited reply")
	if err != nil || revised.Content != "edited reply" || !mutator.revised {
		t.Fatalf("item=%+v err=%v revised=%v", revised, err, mutator.revised)
	}
}

// TestWriteRejectsInvalid 验证空内容、无能力 Repository 和底层错误均不会被吞掉。
func TestWriteRejectsInvalid(t *testing.T) {
	write := NewWrite(&fakeRepo{&fakeMutator{}}, nil)
	if _, err := write.Edit(context.Background(), 1, 2, " "); !errors.Is(err, ErrQuery) {
		t.Fatalf("empty edit error=%v", err)
	}
	if _, err := write.ReviseAssistant(context.Background(), 1, 2, " "); !errors.Is(err, ErrQuery) {
		t.Fatalf("empty revision error=%v", err)
	}
	if err := write.Delete(context.Background(), 0, 2); !errors.Is(err, ErrQuery) {
		t.Fatalf("empty user error=%v", err)
	}
	base := &baseRepo{}
	write = NewWrite(base, nil)
	if _, err := write.Edit(context.Background(), 1, 2, "ok"); !errors.Is(err, ErrQuery) {
		t.Fatalf("missing mutator error=%v", err)
	}
	if _, err := write.ReviseAssistant(context.Background(), 1, 2, "ok"); !errors.Is(err, ErrQuery) {
		t.Fatalf("missing reviser error=%v", err)
	}
}

// TestWriteParent 验证父消息必须属于当前会话。
func TestWriteParent(t *testing.T) {
	repo := &parentRepo{fakeRepo: fakeRepo{&fakeMutator{}}, owned: false}
	write := NewWrite(repo, fakeChat{})
	parent := uint64(9)
	_, err := write.Create(context.Background(), 1, 2, domain.Message{
		ConversationID: 2, ParentID: &parent, Content: "reply",
	})
	if !errors.Is(err, ErrQuery) {
		t.Fatalf("cross chat parent error=%v", err)
	}
	repo.owned = true
	item, err := write.Create(context.Background(), 1, 2, domain.Message{
		ConversationID: 2, ParentID: &parent, Content: "reply",
	})
	if err != nil || item.ParentID == nil {
		t.Fatalf("valid parent item=%+v err=%v", item, err)
	}
}

// TestWriteParentError 验证父消息查询错误必须向上返回。
func TestWriteParentError(t *testing.T) {
	cause := errors.New("parent lookup failed")
	repo := &parentRepo{fakeRepo: fakeRepo{&fakeMutator{}}, err: cause}
	write := NewWrite(repo, fakeChat{})
	parent := uint64(9)
	_, err := write.Create(context.Background(), 1, 2, domain.Message{
		ConversationID: 2, ParentID: &parent, Content: "reply",
	})
	if !errors.Is(err, cause) {
		t.Fatalf("parent error=%v", err)
	}
}

// baseRepo 只满足读写基础接口，用于验证变更能力不可用时安全失败。
type baseRepo struct{}

// List 返回空历史。
func (*baseRepo) List(context.Context, uint64, uint64, int, int) ([]domain.Message, int64, error) {
	return nil, 0, nil
}

// Create 返回输入消息。
func (*baseRepo) Create(_ context.Context, _ uint64, item domain.Message) (domain.Message, error) {
	return item, nil
}
