// write_test.go 验证会话写入用例的标题校验和错误传递。
package app

import (
	"context"
	"errors"
	"testing"

	"ai-chat/backend/internal/chat/domain"
)

// fakeChatRepo 是会话写入用例的最小仓储桩。
type fakeChatRepo struct {
	title string
	err   error
	calls int
}

// List 实现会话列表查询接口。
func (f *fakeChatRepo) List(context.Context, uint64, int, int) ([]domain.Conversation, int64, error) {
	return nil, 0, nil
}

// Find 实现会话详情查询接口。
func (f *fakeChatRepo) Find(context.Context, uint64, uint64) (domain.Conversation, error) {
	return domain.Conversation{}, nil
}

// Owns 实现会话归属查询接口。
func (f *fakeChatRepo) Owns(context.Context, uint64, uint64) (bool, error) {
	return true, nil
}

// Create 实现会话创建接口。
func (f *fakeChatRepo) Create(context.Context, domain.Conversation) (domain.Conversation, error) {
	return domain.Conversation{}, nil
}

// UpdateTitle 记录标题更新参数并返回预设错误。
func (f *fakeChatRepo) UpdateTitle(_ context.Context, _ uint64, _ uint64, title string) error {
	f.calls++
	f.title = title
	return f.err
}

// Delete 实现会话删除接口。
func (f *fakeChatRepo) Delete(context.Context, uint64, uint64) error {
	return nil
}

// TestRename 验证标题会被清理后传给仓储。
func TestRename(t *testing.T) {
	repo := &fakeChatRepo{}
	err := NewWrite(repo, nil).Rename(context.Background(), 1, 2, "  新标题  ")
	if err != nil || repo.title != "新标题" || repo.calls != 1 {
		t.Fatalf("err=%v title=%q calls=%d", err, repo.title, repo.calls)
	}
}

// TestRenameEmpty 验证空标题会被拒绝且不访问仓储。
func TestRenameEmpty(t *testing.T) {
	repo := &fakeChatRepo{}
	err := NewWrite(repo, nil).Rename(context.Background(), 1, 2, " \t ")
	if err == nil || repo.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, repo.calls)
	}
}

// TestRenameRepoErr 验证仓储错误会直接返回。
func TestRenameRepoErr(t *testing.T) {
	want := errors.New("write failed")
	repo := &fakeChatRepo{err: want}
	err := NewWrite(repo, nil).Rename(context.Background(), 1, 2, "标题")
	if !errors.Is(err, want) {
		t.Fatalf("err=%v want=%v", err, want)
	}
}
