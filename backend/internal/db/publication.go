// publication.go 为公共故事入口增加显式、可复核的版本迁移记录。
package db

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/model"
	"crypto/sha256"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const publicationVersion = "005_story_publication"
const publicationPlan = "v1: stories.published_version_id,stories.published_at"

func migratePublication(conn *gorm.DB) error {
	if err := preparePublication(conn); err != nil {
		zap.L().Error("publication schema prepare failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if err := conn.AutoMigrate(&model.Story{}); err != nil {
		zap.L().Error("publication schema columns failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if !conn.Migrator().HasColumn(&model.Story{}, "published_version_id") ||
		!conn.Migrator().HasColumn(&model.Story{}, "published_at") {
		return errors.New("publication columns missing")
	}
	return conn.Model(&schemaChange{}).Where("version = ?", publicationVersion).Update("state", "applied").Error
}

func preparePublication(conn *gorm.DB) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(publicationPlan)))
	var row schemaChange
	err := conn.Where("version = ?", publicationVersion).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conn.Create(&schemaChange{Version: publicationVersion, Digest: digest, State: "started"}).Error
	}
	if err != nil {
		zap.L().Error("publication schema record failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if row.Digest != digest || (row.State != "started" && row.State != "applied") {
		return errors.New("publication schema record mismatch")
	}
	return nil
}
