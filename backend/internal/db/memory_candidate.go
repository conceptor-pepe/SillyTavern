// memory_candidate.go 为用户确认式自动记忆增加版本化迁移。
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

const memoryCandidateVersion = "007_memory_candidates"
const memoryCandidatePlan = "v1: memory_candidates unique(user_id,chat_id,fingerprint); pending review before memory"

func migrateMemoryCandidate(conn *gorm.DB) error {
	if err := prepareMemoryCandidate(conn); err != nil {
		zap.L().Error("memory candidate schema prepare failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if err := conn.AutoMigrate(&model.MemoryCandidate{}); err != nil {
		zap.L().Error("memory candidate schema table failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if !conn.Migrator().HasIndex(&model.MemoryCandidate{}, "uk_candidate_fingerprint") {
		return errors.New("memory candidate index missing")
	}
	return conn.Model(&schemaChange{}).Where("version = ?", memoryCandidateVersion).Update("state", "applied").Error
}

func prepareMemoryCandidate(conn *gorm.DB) error {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(memoryCandidatePlan)))
	var row schemaChange
	err := conn.Where("version = ?", memoryCandidateVersion).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return conn.Create(&schemaChange{Version: memoryCandidateVersion, Digest: digest, State: "started"}).Error
	}
	if err != nil {
		zap.L().Error("memory candidate schema record failed", zap.Error(logx.SafeError(err)))
		return err
	}
	if row.Digest != digest || (row.State != "started" && row.State != "applied") {
		return errors.New("memory candidate schema record mismatch")
	}
	return nil
}
