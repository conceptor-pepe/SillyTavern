// branch_test.go 验证祖先链顺序、完整性、上下文窗口和错误透传。
package app

import (
	"context"
	"errors"
	"testing"

	msgdomain "ai-chat/backend/internal/message/domain"
)

// branchRepo 模拟一次批量分支查询，隔离用例规则与 SQL 实现。
type branchRepo struct {
	items []msgdomain.Message
	err   error
}

// Branch 返回指定链条，校验调用者传递的用户、会话和叶节点。
func (r branchRepo) Branch(_ context.Context, uid, chatID, id uint64) ([]msgdomain.Message, error) {
	if uid != 7 || chatID != 3 || id != uint64(len(r.items)) {
		return nil, ErrBranch
	}
	return r.items, r.err
}

// branchItems 创建已完成的线性链，叶节点为待回复的用户消息。
func branchItems(count int) []msgdomain.Message {
	items := make([]msgdomain.Message, count)
	for i := range items {
		item := msgdomain.Message{
			ID: uint64(i + 1), ConversationID: 3, Role: "user", Content: "question", Status: "completed",
		}
		if i > 0 {
			parent := uint64(i)
			item.ParentID = &parent
		}
		items[i] = item
	}
	return items
}

// TestBranchBounds 验证满窗截断只允许缺省更早祖先，重复内容不额外追加。
func TestBranchBounds(t *testing.T) {
	for _, count := range []int{1, 3, 100} {
		items := branchItems(count)
		if count == 100 {
			older := uint64(999)
			items[0].ParentID = &older
		}
		args := BranchArgs{UserID: 7, ChatID: 3, LeafID: uint64(count), Content: "question"}
		got, err := LoadBranch(context.Background(), branchRepo{items: items}, args)
		if err != nil || len(got) != count {
			t.Fatalf("count=%d got=%d err=%v", count, len(got), err)
		}
	}
}

// TestBranchRejects 验证脏链不能进入模型上下文。
func TestBranchRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func([]msgdomain.Message)
	}{
		{"cross chat", func(items []msgdomain.Message) { items[0].ConversationID = 8 }},
		{"unfinished", func(items []msgdomain.Message) { items[0].Status = "failed" }},
		{"wrong role", func(items []msgdomain.Message) { items[2].Role = "assistant" }},
		{"empty user", func(items []msgdomain.Message) { items[2].Content = " " }},
		{"broken link", func(items []msgdomain.Message) { items[1].ParentID = nil }},
		{"missing root", func(items []msgdomain.Message) { id := uint64(9); items[0].ParentID = &id }},
		{"cycle", func(items []msgdomain.Message) { id := uint64(3); items[0].ParentID = &id }},
		{"duplicate", func(items []msgdomain.Message) { items[0].ID = 2 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := branchItems(3)
			tc.edit(items)
			err := checkBranch(items, BranchArgs{ChatID: 3, LeafID: 3})
			if !errors.Is(err, ErrBranch) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

// TestBranchErrors 验证内容冲突、错叶节点、空链和底层错误均被保留。
func TestBranchErrors(t *testing.T) {
	for _, args := range []BranchArgs{
		{ChatID: 3, LeafID: 2}, {ChatID: 3, LeafID: 3, Content: "different"},
	} {
		if err := checkBranch(branchItems(3), args); !errors.Is(err, ErrBranch) {
			t.Fatalf("args=%+v error=%v", args, err)
		}
	}
	for _, count := range []int{0, 101} {
		if err := checkBranch(branchItems(count), BranchArgs{}); !errors.Is(err, ErrBranch) {
			t.Fatalf("count=%d error=%v", count, err)
		}
	}
	cause := errors.New("query failed")
	args := BranchArgs{UserID: 7, ChatID: 3, LeafID: 1}
	_, err := LoadBranch(context.Background(), branchRepo{items: branchItems(1), err: cause}, args)
	if !errors.Is(err, cause) {
		t.Fatalf("error=%v", err)
	}
}
