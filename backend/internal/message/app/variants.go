// variants.go 负责候选历史的消息可见性校验和分页查询。
package app

import (
	"context"
	"errors"

	"ai-chat/backend/internal/message/domain"
)

// Variants 仅查询当前用户可见消息的候选，不为单候选旧消息虚构记录。
func (q *Query) Variants(ctx context.Context, uid, messageID uint64, page, size int) ([]domain.Variant, int64, error) {
	if uid == 0 || messageID == 0 || page < 1 || size < 1 || size > 100 {
		return nil, 0, ErrQuery
	}
	finder, found := q.messages.(domain.Finder)
	variants, supported := q.messages.(domain.VariantRepo)
	if !found || !supported {
		return nil, 0, errors.New("variant query unavailable")
	}
	if _, err := finder.Find(ctx, uid, messageID); err != nil {
		return nil, 0, err
	}
	return variants.ListVariants(ctx, uid, messageID, page, size)
}
