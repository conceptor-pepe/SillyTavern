// select.go 负责选择候选回复用例，保持原消息及其已有后续分支不变。
package app

import (
	"context"
	"errors"

	"ai-chat/backend/internal/message/domain"
)

// SelectVariant 校验请求后交给原子选择能力，不使用先查后写的分离事务。
func (w *Write) SelectVariant(ctx context.Context, uid, messageID, variantID uint64) (domain.Message, error) {
	if uid == 0 || messageID == 0 || variantID == 0 {
		return domain.Message{}, ErrQuery
	}
	selector, ok := w.messages.(domain.VariantSelector)
	if !ok {
		return domain.Message{}, errors.New("variant selector unavailable")
	}
	return selector.SelectVariant(ctx, uid, messageID, variantID)
}
