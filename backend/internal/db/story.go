// story.go 在现有结构锁内执行可重试的增量故事迁移。
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

const storyVersion = "004_story_sessions"
const storyPlan = "v1: stories,story_versions,player_personas,story_sessions; conversations.mode=legacy; memories.chat_id=0"

func migrateStory(conn *gorm.DB) error {
	if err := prepareStory(conn); err != nil {
		zap.L().Error("story schema prepare failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if err := conn.AutoMigrate(&model.Story{}, &model.StoryVersion{}, &model.PlayerPersona{}, &model.StorySession{}); err != nil {
		zap.L().Error("story schema tables failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if err := verifyStory(conn); err != nil {
		zap.L().Error("story schema verification failed", zap.Error(logx.SafeError(err)))
		return err
	}
	return conn.Model(&schemaChange{}).Where("version = ?", storyVersion).Update("state", "applied").Error
}

func prepareStory(conn *gorm.DB) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(storyPlan)))
	var row schemaChange
	err := conn.Where("version = ?", storyVersion).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conn.Create(&schemaChange{Version: storyVersion, Digest: digest, State: "started"}).Error
	}
	if err != nil {
		zap.L().Error("story schema record failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if row.Digest != digest || (row.State != "started" && row.State != "applied") {
		return errors.New("story schema record mismatch")
	}
	return nil
}

func verifyStory(conn *gorm.DB) error {
	m := conn.Migrator()
	if !m.HasColumn(&model.Conversation{}, "mode") || !m.HasColumn(&model.Memory{}, "chat_id") {
		return errors.New("story scope columns missing")
	}
	if !m.HasIndex(&model.StorySession{}, "uk_story_start") || !m.HasIndex(&model.StoryVersion{}, "uk_story_revision") {
		return errors.New("story idempotency indexes missing")
	}
	return nil
}
