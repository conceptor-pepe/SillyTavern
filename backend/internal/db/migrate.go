// migrate.go 负责创建和升级 AI Chat 的核心数据表。
package db

import (
	"context"

	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Migrate 串行创建基础表并执行有版本记录的修复，DDL 不伪装成可整体回滚事务。
func Migrate(ctx context.Context, conn *gorm.DB) error {
	return schemaLock(ctx, conn, func(conn *gorm.DB) error {
		if err := prepareChange(conn); err != nil {
			zap.L().Error("schema migration failed", zap.Error(logx.SafeError(err)))
			return err
		}
		if err := checkLegacy(conn); err != nil {
			zap.L().Error("schema migration failed", zap.Error(logx.SafeError(err)))
			return err
		}
		if err := createTables(ctx, conn); err != nil {
			zap.L().Error("schema migration failed", zap.Error(logx.SafeError(err)))
			return err
		}
		if err := migrateStory(conn); err != nil {
			zap.L().Error("story migration failed", zap.Error(logx.SafeError(err)))
			return err
		}
		if err := migratePublication(conn); err != nil {
			zap.L().Error("story publication migration failed", zap.Error(logx.SafeError(err)))
			return err
		}
		if err := migrateRelationship(conn); err != nil {
			zap.L().Error("relationship migration failed", zap.Error(logx.SafeError(err)))
			return err
		}
		if err := migrateMemoryCandidate(conn); err != nil {
			zap.L().Error("memory candidate migration failed", zap.Error(logx.SafeError(err)))
			return err
		}
		return fixFavorites(conn)
	})
}

// createTables 暂时保留既有建表路径，版本化基线迁移替换前不得宣称完整发布迁移已完成。
func createTables(ctx context.Context, conn *gorm.DB) error {
	return conn.WithContext(ctx).AutoMigrate(
		&model.User{},
		&model.UserSetting{},
		&model.Character{},
		&model.Conversation{},
		&model.Favorite{},
		&model.Message{},
		&model.MessageVariant{},
		&model.Generation{},
		&model.Asset{},
		&model.Background{},
		&model.Memory{},
		&model.MemorySummary{},
		&model.Relationship{},
		&model.MemoryCandidate{},
	)
}
