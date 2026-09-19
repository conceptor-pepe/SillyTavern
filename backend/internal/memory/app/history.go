// history.go 逐页读取祖先，不使用时间顺序混合兄弟剧情。
package app

import (
	"ai-chat/backend/internal/logx"
	msg "ai-chat/backend/internal/message/domain"
	"context"
	"errors"
	"go.uber.org/zap"
)

// expand 在近期分支前补齐历史；超过硬上限明确拒绝，不能静默丢失旧约定。
func (s *Service) expand(ctx context.Context, uid uint64, items []msg.Message) ([]msg.Message, error) {
	if len(items) == 0 {
		return items, nil
	}
	for items[0].ParentID != nil {
		if len(items) >= 4000 {
			return nil, errors.New("history exceeds summary scan limit")
		}
		head := items[0]
		page, err := s.history.Branch(ctx, uid, head.ConversationID, *head.ParentID)
		if err != nil {
			s.logger.Warn("memory history failed", zap.Error(logx.SafeError(err)))
			return nil, err
		}
		if len(page) == 0 || page[len(page)-1].ID != *head.ParentID {
			return nil, errors.New("broken memory history")
		}
		items = append(page, items...)
		if err := validHistory(items); err != nil {
			s.logger.Warn("memory history rejected", zap.Error(err))
			return nil, err
		}
	}
	return items, validHistory(items)
}

// validHistory 校验完整快照与消息角色，已删除或重复的祖先不能用于摘要。
func validHistory(items []msg.Message) error {
	seen := make(map[uint64]bool, len(items))
	for i, item := range items {
		if item.ID == 0 || seen[item.ID] || item.Status != "completed" || (item.Role != "user" && item.Role != "assistant" && item.Role != "system") {
			return errors.New("invalid memory history")
		}
		if i > 0 && (item.ParentID == nil || *item.ParentID != items[i-1].ID || item.ConversationID != items[0].ConversationID) {
			return errors.New("invalid memory branch")
		}
		seen[item.ID] = true
	}
	return nil
}
