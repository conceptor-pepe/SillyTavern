// favorite.go 负责会话收藏用例，维护用户范围和幂等规则。
package app

import (
	"context"
	"errors"
)

// FavoriteRepo 提供会话收藏关系的持久化能力。
type FavoriteRepo interface {
	Set(ctx context.Context, userID, chatID uint64, on bool) error
}

// Favorite 保存收藏写入依赖。
type Favorite struct {
	repo FavoriteRepo
}

// NewFavorite 创建收藏用例。
func NewFavorite(repo FavoriteRepo) *Favorite { return &Favorite{repo: repo} }

// Set 设置当前用户的会话收藏状态。
func (f *Favorite) Set(ctx context.Context, userID, chatID uint64, on bool) error {
	if userID == 0 || chatID == 0 {
		return errors.New("invalid favorite")
	}
	if f.repo == nil {
		return errors.New("favorite repository unavailable")
	}
	return f.repo.Set(ctx, userID, chatID, on)
}
