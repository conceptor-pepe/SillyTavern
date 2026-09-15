// recent.go 负责最近聊天查询用例，统一限制用户和分页范围。
package app

import (
	"context"
	"errors"

	"ai-chat/backend/internal/chat/domain"
)

// RecentRepo 提供最近聊天的专用查询能力。
type RecentRepo interface {
	Recent(ctx context.Context, userID uint64, page, size int) ([]domain.Conversation, int64, error)
}

// Recent 保存最近聊天查询依赖。
type Recent struct{ repo RecentRepo }

// NewRecent 创建最近聊天查询用例。
func NewRecent(repo RecentRepo) *Recent { return &Recent{repo: repo} }

// List 查询当前用户最近有消息的会话。
func (r *Recent) List(ctx context.Context, userID uint64, page, size int) (Page, error) {
	if userID == 0 {
		return Page{}, ErrIdentity
	}
	if r.repo == nil {
		return Page{}, errors.New("recent repository unavailable")
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return Page{}, errors.New("invalid recent query")
	}
	items, total, err := r.repo.Recent(ctx, userID, page, size)
	if err != nil {
		return Page{}, err
	}
	if items == nil {
		items = []domain.Conversation{}
	}
	return Page{Items: items, Total: total, Page: page, Size: size}, nil
}
