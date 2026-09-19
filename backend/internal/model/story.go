// story.go 保存草稿与不可变作品版本，不关联可变角色资料。
package model

import "time"

// Story 保存作者私有草稿，Revision 用于并发编辑保护。
type Story struct {
	Base
	UserID             uint64     `gorm:"not null;index"`
	Revision           uint64     `gorm:"not null"`
	Definition         string     `gorm:"type:json;not null"`
	PublishedVersionID *uint64    `gorm:"index"`
	PublishedAt        *time.Time `gorm:"index"`
}

// StoryVersion 冻结同一草稿修订，重复冻结返回同一版本。
type StoryVersion struct {
	Base
	UserID     uint64 `gorm:"not null;index"`
	StoryID    uint64 `gorm:"not null;uniqueIndex:uk_story_revision,priority:1"`
	Revision   uint64 `gorm:"not null;uniqueIndex:uk_story_revision,priority:2"`
	Definition string `gorm:"type:json;not null"`
	Digest     string `gorm:"size:64;not null"`
}

// StorySession 将会话绑定到不可变版本和玩家快照，开聊键按账号唯一。
type StorySession struct {
	Base
	UserID      uint64 `gorm:"not null;uniqueIndex:uk_story_start,priority:1"`
	ChatID      uint64 `gorm:"not null;uniqueIndex"`
	VersionID   uint64 `gorm:"not null;index"`
	CompanionID uint64 `gorm:"not null;default:0;index"`
	PersonaID   uint64 `gorm:"not null"`
	Persona     string `gorm:"type:json;not null"`
	StartKey    string `gorm:"size:64;not null;uniqueIndex:uk_story_start,priority:2"`
	RequestHash string `gorm:"size:64;not null"`
	OpeningID   *uint64
}
