// recent_test.go 验证最近聊天用例的分页和空结果行为。
package app

import (
	"context"
	"testing"

	"ai-chat/backend/internal/chat/domain"
)

// recentRepoStub 是最近聊天查询的测试桩。
type recentRepoStub struct {
	items []domain.Conversation
	total int64
}

// Recent 返回预设的最近聊天结果。
func (r *recentRepoStub) Recent(context.Context, uint64, int, int) ([]domain.Conversation, int64, error) {
	return r.items, r.total, nil
}

// TestRecentList 验证最近聊天结果和分页参数。
func TestRecentList(t *testing.T) {
	page, err := NewRecent(&recentRepoStub{total: 2}).List(context.Background(), 7, 1, 20)
	if err != nil || page.Total != 2 || page.Items == nil {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
