// service.go 维护作品草稿和可玩版本，所有业务结果都有归属日志。
package app

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/story/domain"
	"context"
	"go.uber.org/zap"
)

// Repo 是作品用例所需的最小持久化契约。
type Repo interface {
	List(context.Context, uint64, int) ([]domain.Story, error)
	Find(context.Context, uint64, uint64) (domain.Story, error)
	PublicList(context.Context, int, string) ([]domain.PublicStory, error)
	PublicFind(context.Context, uint64) (domain.PublicStory, error)
	Save(context.Context, domain.Story) (domain.Story, error)
	Delete(context.Context, uint64, uint64) error
	Freeze(context.Context, domain.Story) (domain.Version, error)
	Version(context.Context, uint64, uint64) (domain.Version, error)
	Publish(context.Context, domain.Story) (domain.Story, error)
	Unpublish(context.Context, uint64, uint64) (domain.Story, error)
}

// CompanionRepo 校验作品引用的长期角色仍属于作者。
type CompanionRepo interface {
	OwnsCompanion(context.Context, uint64, uint64) (bool, error)
}

// Service 在持久化前校验完整作品定义。
type Service struct {
	repo   Repo
	logger *zap.Logger
}

// New 创建作品服务。
// @param repo 作品仓储
// @param logger 业务日志
// @return 作品服务
func New(repo Repo, logger *zap.Logger) *Service { return &Service{repo: repo, logger: logger} }

// Save 用 Revision 实现草稿的乐观锁更新。
// @param ctx 请求上下文
// @param item 草稿内容
// @return 保存结果与错误
func (s *Service) Save(ctx context.Context, item domain.Story) (domain.Story, error) {
	if err := domain.Validate(item.Definition); err != nil {
		s.logger.Warn("story validation rejected", zap.Uint64("user_id", item.UserID))
		return item, err
	}
	if err := s.checkCompanion(ctx, item); err != nil {
		s.logger.Warn("story companion rejected", zap.Uint64("user_id", item.UserID), zap.Error(logx.SafeError(err)))
		return item, err
	}
	out, err := s.repo.Save(ctx, item)
	s.record("story saved", item.UserID, err)
	return out, err
}

func (s *Service) checkCompanion(ctx context.Context, item domain.Story) error {
	id := item.Definition.Cast[0].CompanionID
	if id == 0 {
		return nil
	}
	repo, ok := s.repo.(CompanionRepo)
	if !ok {
		return domain.ErrInvalid
	}
	owned, err := repo.OwnsCompanion(ctx, item.UserID, id)
	if err != nil {
		s.logger.Error("story companion ownership failed", zap.Uint64("user_id", item.UserID), zap.Uint64("companion_id", id), zap.Error(logx.SafeError(err)))
		return err
	}
	if !owned {
		return domain.ErrInvalid
	}
	return nil
}

func (s *Service) record(op string, uid uint64, err error) {
	if err != nil {
		s.logger.Warn(op+" failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return
	}
	s.logger.Info(op, zap.Uint64("user_id", uid))
}
