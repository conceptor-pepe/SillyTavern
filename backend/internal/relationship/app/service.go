// service.go 校验并维护跨故事共享的关系档案。
package app

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/relationship/domain"
	"context"
	"strings"

	"go.uber.org/zap"
)

var stages = map[string]bool{"acquaintance": true, "friend": true, "close": true, "romantic": true, "committed": true}

// Service 提供关系档案读取和乐观锁更新。
type Service struct {
	repo   domain.Repo
	logger *zap.Logger
}

// New 创建关系服务。
// @param repo 关系仓储
// @param logger 业务日志
// @return 关系服务
func New(repo domain.Repo, logger *zap.Logger) *Service { return &Service{repo: repo, logger: logger} }

// ByChat 读取当前会话对应的共享关系。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @return 关系档案与错误
func (s *Service) ByChat(ctx context.Context, uid, chatID uint64) (domain.Relationship, error) {
	out, err := s.repo.ByChat(ctx, uid, chatID)
	if err != nil {
		s.logger.Warn("relationship read failed", zap.Uint64("user_id", uid), zap.Uint64("chat_id", chatID), zap.Error(logx.SafeError(err)))
	}
	return out, err
}

// UpdateByChat 更新所有故事共享的关系描述和里程碑。
// @param ctx 请求上下文
// @param in 更新命令
// @return 关系档案与错误
func (s *Service) UpdateByChat(ctx context.Context, in domain.Update) (domain.Relationship, error) {
	clean(&in)
	if !valid(in) {
		s.logger.Warn("relationship update rejected", zap.Uint64("user_id", in.UserID), zap.Uint64("chat_id", in.ChatID))
		return domain.Relationship{}, domain.ErrInvalid
	}
	out, err := s.repo.UpdateByChat(ctx, in)
	if err != nil {
		s.logger.Warn("relationship update failed", zap.Uint64("user_id", in.UserID), zap.Uint64("chat_id", in.ChatID), zap.Error(logx.SafeError(err)))
		return out, err
	}
	s.logger.Info("relationship updated", zap.Uint64("user_id", in.UserID), zap.Uint64("companion_id", out.CompanionID))
	return out, nil
}

func clean(in *domain.Update) {
	in.Stage = strings.TrimSpace(in.Stage)
	in.Narrative = strings.TrimSpace(in.Narrative)
	for i := range in.Milestones {
		in.Milestones[i] = strings.TrimSpace(in.Milestones[i])
	}
}

func valid(in domain.Update) bool {
	if in.UserID == 0 || in.ChatID == 0 || in.Revision == 0 || !stages[in.Stage] || len(in.Narrative) > 4000 || len(in.Milestones) > 32 {
		return false
	}
	total := 0
	for _, item := range in.Milestones {
		if item == "" || len(item) > 300 {
			return false
		}
		total += len(item)
	}
	return total <= 4000
}
