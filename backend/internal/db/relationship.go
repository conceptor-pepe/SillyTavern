// relationship.go 为稳定角色关系增加可重试迁移与旧会话回填。
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

const relationshipVersion = "006_companion_relationships"
const relationshipPlan = "v1: relationships(user_id,companion_id); story_sessions.companion_id; backfill legacy conversations"

func migrateRelationship(conn *gorm.DB) error {
	if err := prepareRelationship(conn); err != nil {
		zap.L().Error("relationship schema prepare failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if err := conn.AutoMigrate(&model.Relationship{}, &model.StorySession{}); err != nil {
		zap.L().Error("relationship schema tables failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if err := backfillRelationships(conn); err != nil {
		zap.L().Error("relationship backfill failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if !conn.Migrator().HasIndex(&model.Relationship{}, "uk_user_companion") ||
		!conn.Migrator().HasColumn(&model.StorySession{}, "companion_id") {
		return errors.New("relationship schema verification failed")
	}
	return conn.Model(&schemaChange{}).Where("version = ?", relationshipVersion).Update("state", "applied").Error
}

func prepareRelationship(conn *gorm.DB) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(relationshipPlan)))
	var row schemaChange
	err := conn.Where("version = ?", relationshipVersion).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conn.Create(&schemaChange{Version: relationshipVersion, Digest: digest, State: "started"}).Error
	}
	if err != nil {
		zap.L().Error("relationship schema record failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if row.Digest != digest || (row.State != "started" && row.State != "applied") {
		return errors.New("relationship schema record mismatch")
	}
	return nil
}

func backfillRelationships(conn *gorm.DB) error {
	return conn.Exec(`INSERT IGNORE INTO relationships
		(user_id, companion_id, stage, narrative, milestones, revision, created_at, updated_at)
		SELECT DISTINCT user_id, character_id, 'acquaintance', '', JSON_ARRAY(), 1, UTC_TIMESTAMP(), UTC_TIMESTAMP()
		FROM conversations
		WHERE character_id > 0 AND deleted_at IS NULL`).Error
}
