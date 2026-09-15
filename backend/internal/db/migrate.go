// migrate.go 负责创建和升级 AI Chat 的核心数据表。
package db

import (
	"context"

	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// Migrate 创建第一阶段所需的全部 GORM 数据表。
func Migrate(ctx context.Context, conn *gorm.DB) error {
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
	)
}
