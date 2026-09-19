// service.go 为玩家身份提供验证和可追踪的应用入口。
package app

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/persona/domain"
	"context"
	"go.uber.org/zap"
)

// Repo 限制人设操作在用户范围内。
type Repo interface {
	List(context.Context, uint64, int) ([]domain.Persona, error)
	Find(context.Context, uint64, uint64) (domain.Persona, error)
	Save(context.Context, domain.Persona) (domain.Persona, error)
	Delete(context.Context, uint64, uint64) error
}

// Service 执行玩家身份用例。
type Service struct {
	repo   Repo
	logger *zap.Logger
}

// New 创建玩家身份服务。
// @param repo 人设仓储
// @param logger 业务日志
// @return 玩家服务
func New(repo Repo, logger *zap.Logger) *Service { return &Service{repo: repo, logger: logger} }

// Save 校验并保存带修订的人设。
// @param ctx 请求上下文
// @param p 玩家人设
// @return 人设与错误
func (s *Service) Save(ctx context.Context, p domain.Persona) (domain.Persona, error) {
	if err := domain.Validate(p); err != nil {
		s.logger.Warn("persona rejected", zap.Uint64("user_id", p.UserID))
		return p, err
	}
	out, err := s.repo.Save(ctx, p)
	s.record("persona saved", p.UserID, err)
	return out, err
}

// List 返回本人身份的固定分页。
// @param ctx 请求上下文
// @param uid 用户编号
// @param page 页码
// @return 人设与错误
func (s *Service) List(ctx context.Context, uid uint64, page int) ([]domain.Persona, error) {
	out, err := s.repo.List(ctx, uid, page)
	s.record("persona list", uid, err)
	return out, err
}

// Find 读取本人仍可用的人设。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 人设编号
// @return 人设与错误
func (s *Service) Find(ctx context.Context, uid, id uint64) (domain.Persona, error) {
	out, err := s.repo.Find(ctx, uid, id)
	s.record("persona read", uid, err)
	return out, err
}

// Delete 软删除人设，历史快照不受影响。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 人设编号
// @return 删除错误
func (s *Service) Delete(ctx context.Context, uid, id uint64) error {
	err := s.repo.Delete(ctx, uid, id)
	s.record("persona deleted", uid, err)
	return err
}

func (s *Service) record(op string, uid uint64, err error) {
	if err != nil {
		s.logger.Warn(op+" failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return
	}
	s.logger.Info(op, zap.Uint64("user_id", uid))
}
