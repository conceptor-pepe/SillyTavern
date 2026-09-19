// story.go 编排故事开聊，仓储以事务保存会话和开场。
package app

import (
	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/logx"
	"context"
	"go.uber.org/zap"
	"regexp"
)

var startKey = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,64}$`)

// StoryRepo 提供原子开聊与快照读取。
type StoryRepo interface {
	Start(context.Context, domain.StoryStart) (domain.StoryState, error)
	State(context.Context, uint64, uint64) (domain.StoryState, error)
}

// StoryService 不允许客户端构造任意版本或玩家快照。
type StoryService struct {
	repo   StoryRepo
	logger *zap.Logger
}

// NewStory 创建故事会话服务。
// @param repo 会话仓储
// @param logger 业务日志
// @return 会话服务
func NewStory(repo StoryRepo, logger *zap.Logger) *StoryService {
	return &StoryService{repo: repo, logger: logger}
}

// Start 幂等创建当前用户的独立故事存档。
// @param ctx 请求上下文
// @param in 开聊参数
// @return 会话快照与错误
func (s *StoryService) Start(ctx context.Context, in domain.StoryStart) (domain.StoryState, error) {
	if in.UserID == 0 || in.VersionID == 0 || in.PersonaID == 0 || !startKey.MatchString(in.Key) {
		s.logger.Warn("story session rejected", zap.Uint64("user_id", in.UserID))
		return domain.StoryState{}, domain.ErrStoryInput
	}
	out, err := s.repo.Start(ctx, in)
	if err != nil {
		s.logger.Warn("story start failed", zap.Uint64("user_id", in.UserID), zap.Error(logx.SafeError(err)))
		return out, err
	}
	s.logger.Info("story started", zap.Uint64("user_id", in.UserID), zap.Uint64("chat_id", out.Chat.ID))
	return out, nil
}

// State 只返回本人未删除会话的冻结资料。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 会话编号
// @return 会话快照与错误
func (s *StoryService) State(ctx context.Context, uid, id uint64) (domain.StoryState, error) {
	out, err := s.repo.State(ctx, uid, id)
	if err != nil {
		s.logger.Warn("story state failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
	}
	return out, err
}
