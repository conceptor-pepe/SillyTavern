// user.go 负责将来源聊天目录与迁移账号匹配，防止跨用户导入。
package main

import (
	"context"
	"errors"
	"path/filepath"

	migrationapp "ai-chat/backend/internal/migration/app"
	legacy "ai-chat/backend/internal/migration/domain"
	migrationinfra "ai-chat/backend/internal/migration/infra"
	"go.uber.org/zap"
)

// sourceUser 根据旧版 data/<handle>/chats 布局定位唯一来源账号。
func sourceUser(root string) (legacy.User, error) {
	path, err := filepath.Abs(root)
	if err != nil {
		return legacy.User{}, err
	}
	if filepath.Base(path) != "chats" {
		return legacy.User{}, errors.New("automatic user matching requires data/<handle>/chats")
	}
	userRoot := filepath.Dir(path)
	storage := filepath.Join(filepath.Dir(userRoot), "_storage")
	return migrationinfra.FindUser(storage, filepath.Base(userRoot))
}

// importUser 自动匹配来源账号；显式用户编号表示操作者指定的目标映射。
func importUser(ctx context.Context, root string, userID uint64, writer migrationapp.UserWriter, logger *zap.Logger) uint64 {
	if userID != 0 {
		logger.Info("migration user override", zap.Uint64("user_id", userID))
		return userID
	}
	item, err := sourceUser(root)
	if err != nil {
		logger.Error("migration source user invalid", zap.String("path", root), zap.Error(err))
		return 0
	}
	created, id, err := writer.ImportUser(ctx, item)
	if err != nil {
		logger.Error("user migration failed", zap.String("handle", item.Handle), zap.Error(err))
		return 0
	}
	logger.Info("user migration finished", zap.String("handle", item.Handle),
		zap.Uint64("user_id", id), zap.Bool("created", created))
	return id
}
