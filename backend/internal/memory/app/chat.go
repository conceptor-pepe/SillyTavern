// chat.go 将故事记忆限制在当前存档内，不继承素材角色的事实。
package app

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/memory/domain"
	"context"
	"go.uber.org/zap"
)

func (s *Service) scopeEntries(ctx context.Context, item domain.Entry) ([]domain.Entry, error) {
	if item.ChatID != 0 {
		return s.ListChat(ctx, item.UserID, item.ChatID)
	}
	return s.repo.List(ctx, item.UserID, item.CharacterID)
}

// ListChat 读取当前用户故事存档的独立记忆。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @return 记忆与错误
func (s *Service) ListChat(ctx context.Context, uid, chatID uint64) ([]domain.Entry, error) {
	repo, ok := s.repo.(domain.ChatRepo)
	if !ok {
		return nil, domain.ErrNotFound
	}
	out, err := repo.ListChat(ctx, uid, chatID)
	if err != nil {
		s.logger.Warn("chat memory read failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
	}
	return out, err
}

// DeleteChat 立即移除当前存档的指定记忆。
// @param ctx 请求上下文
// @param item 记忆归属
// @return 删除错误
func (s *Service) DeleteChat(ctx context.Context, item domain.Entry) error {
	repo, ok := s.repo.(domain.ChatRepo)
	if !ok {
		return domain.ErrNotFound
	}
	err := repo.DeleteChat(ctx, item.UserID, item.ChatID, item.ID)
	if err != nil {
		s.logger.Warn("chat memory delete failed", zap.Uint64("user_id", item.UserID), zap.Error(logx.SafeError(err)))
		return err
	}
	s.logger.Info("chat memory deleted", zap.Uint64("user_id", item.UserID), zap.Uint64("chat_id", item.ChatID))
	return nil
}

// ListRelationship 读取同一角色在所有故事之间共享的记忆。
// @param ctx 请求上下文
// @param uid 用户编号
// @param companionID 角色编号
// @return 记忆与错误
func (s *Service) ListRelationship(ctx context.Context, uid, companionID uint64) ([]domain.Entry, error) {
	repo, ok := s.repo.(domain.RelationshipRepo)
	if !ok {
		return nil, domain.ErrNotFound
	}
	out, err := repo.ListRelationship(ctx, uid, companionID)
	if err != nil {
		s.logger.Warn("relationship memory read failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
	}
	return out, err
}

// SaveRelationship 保存经过关系授权的跨故事记忆。
// @param ctx 请求上下文
// @param item 记忆内容
// @return 保存结果与错误
func (s *Service) SaveRelationship(ctx context.Context, item domain.Entry) (domain.Entry, error) {
	if err := validate(&item); err != nil {
		s.logger.Warn("relationship memory rejected", zap.Error(err))
		return item, err
	}
	repo, ok := s.repo.(domain.RelationshipRepo)
	if !ok {
		return item, domain.ErrNotFound
	}
	entries, err := repo.ListRelationship(ctx, item.UserID, item.CharacterID)
	if err != nil {
		s.logger.Warn("relationship memory scope rejected", zap.Error(logx.SafeError(err)))
		return item, err
	}
	if pinnedSize(entries, item) > 6000 {
		return item, domain.ErrInvalid
	}
	item.Vector, item.VectorModel = nil, ""
	if s.embed != nil {
		s.index(ctx, &item)
	}
	out, err := repo.SaveRelationship(ctx, item)
	if err != nil {
		s.logger.Error("relationship memory save failed", zap.Error(logx.SafeError(err)))
		return out, err
	}
	s.logger.Info("relationship memory saved", zap.Uint64("user_id", item.UserID), zap.Uint64("companion_id", item.CharacterID))
	return out, nil
}

// DeleteRelationship 删除关系范围内的记忆。
// @param ctx 请求上下文
// @param item 记忆归属
// @return 删除错误
func (s *Service) DeleteRelationship(ctx context.Context, item domain.Entry) error {
	repo, ok := s.repo.(domain.RelationshipRepo)
	if !ok {
		return domain.ErrNotFound
	}
	err := repo.DeleteRelationship(ctx, item.UserID, item.CharacterID, item.ID)
	if err != nil {
		s.logger.Warn("relationship memory delete failed", zap.Error(logx.SafeError(err)))
		return err
	}
	s.logger.Info("relationship memory deleted", zap.Uint64("user_id", item.UserID), zap.Uint64("companion_id", item.CharacterID))
	return nil
}
