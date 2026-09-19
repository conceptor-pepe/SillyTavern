// query.go 将作品读取和版本冻结交给同一归属边界。
package app

import (
	"ai-chat/backend/internal/story/domain"
	"context"
)

// List 返回当前用户的有界草稿列表。
// @param ctx 请求上下文
// @param uid 用户编号
// @param page 页码
// @return 草稿与错误
func (s *Service) List(ctx context.Context, uid uint64, page int) ([]domain.Story, error) {
	out, err := s.repo.List(ctx, uid, page)
	s.record("story list", uid, err)
	return out, err
}

// Find 读取当前用户的草稿。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 作品编号
// @return 草稿与错误
func (s *Service) Find(ctx context.Context, uid, id uint64) (domain.Story, error) {
	out, err := s.repo.Find(ctx, uid, id)
	s.record("story read", uid, err)
	return out, err
}

// PublicList 返回所有用户可见的冻结作品。
// @param ctx 请求上下文
// @param page 页码
// @param query 标题或标签关键词
// @return 公开作品与错误
func (s *Service) PublicList(ctx context.Context, page int, query string) ([]domain.PublicStory, error) {
	out, err := s.repo.PublicList(ctx, page, query)
	s.record("public story list", 0, err)
	return out, err
}

// PublicFind 返回指定公开作品的当前发布版本。
// @param ctx 请求上下文
// @param id 作品编号
// @return 公开作品与错误
func (s *Service) PublicFind(ctx context.Context, id uint64) (domain.PublicStory, error) {
	out, err := s.repo.PublicFind(ctx, id)
	s.record("public story read", 0, err)
	return out, err
}

// Delete 停止新开故事，但保留版本供已有会话继续。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 作品编号
// @return 删除错误
func (s *Service) Delete(ctx context.Context, uid, id uint64) error {
	err := s.repo.Delete(ctx, uid, id)
	s.record("story deleted", uid, err)
	return err
}

// Freeze 幂等冻结指定草稿修订，不修改既有版本。
// @param ctx 请求上下文
// @param item 作品及修订
// @return 版本与错误
func (s *Service) Freeze(ctx context.Context, item domain.Story) (domain.Version, error) {
	out, err := s.repo.Freeze(ctx, item)
	s.record("story frozen", item.UserID, err)
	return out, err
}

// Version 返回作者仍可见作品的指定版本。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 版本编号
// @return 版本与错误
func (s *Service) Version(ctx context.Context, uid, id uint64) (domain.Version, error) {
	out, err := s.repo.Version(ctx, uid, id)
	s.record("story version read", uid, err)
	return out, err
}

// Publish 冻结当前修订并将其设为公开版本。
// @param ctx 请求上下文
// @param item 作品及预期修订
// @return 更新后的作品与错误
func (s *Service) Publish(ctx context.Context, item domain.Story) (domain.Story, error) {
	out, err := s.repo.Publish(ctx, item)
	s.record("story published", item.UserID, err)
	return out, err
}

// Unpublish 撤下公开入口，已创建的会话仍可读取冻结版本。
// @param ctx 请求上下文
// @param uid 作者编号
// @param id 作品编号
// @return 更新后的作品与错误
func (s *Service) Unpublish(ctx context.Context, uid, id uint64) (domain.Story, error) {
	out, err := s.repo.Unpublish(ctx, uid, id)
	s.record("story unpublished", uid, err)
	return out, err
}
