// schema_change.go 持久化收藏索引升级的版本和状态，中断后先检查结构再重试。
package db

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const favoriteVersion = "003_favorite_owner"
const favoritePlan = "v1: favorites unique(user_id,character_id,kind); remove uk_user_char_kind"

// schemaChange 是技术升级记录，不属于任何业务域。
type schemaChange struct {
	Version   string    `gorm:"size:64;primaryKey"`
	Digest    string    `gorm:"size:64;not null"`
	State     string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// prepareChange 先检查版本摘要，防止已发布的升级被无声改写；不记录底层敏感错误。
func prepareChange(conn *gorm.DB) error {
	if err := conn.AutoMigrate(&schemaChange{}); err != nil {
		return err
	}
	var row schemaChange
	err := conn.Where("version = ?", favoriteVersion).First(&row).Error
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(favoritePlan)))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now().UTC()
		return conn.Create(&schemaChange{Version: favoriteVersion, Digest: digest, State: "started",
			CreatedAt: now, UpdatedAt: now}).Error
	}
	if err != nil {
		return err
	}
	if row.Digest != digest || (row.State != "started" && row.State != "applied") {
		return errors.New("schema change record mismatch")
	}
	if row.State == "applied" {
		return checkFavorites(conn)
	}
	return nil
}

// markChange 只在结构校验成功后记录完成；DDL 已提交但记录失败时允许安全重试。
func markChange(conn *gorm.DB) error {
	return conn.Model(&schemaChange{}).Where("version = ? AND state = ?", favoriteVersion, "started").
		Updates(map[string]any{"state": "applied", "updated_at": time.Now().UTC()}).Error
}
