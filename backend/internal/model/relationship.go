// relationship.go 保存用户与稳定角色身份之间的连续关系。
package model

// Relationship 在所有故事和会话之间共享，不绑定单个聊天分支。
type Relationship struct {
	Base
	UserID      uint64 `gorm:"not null;uniqueIndex:uk_user_companion,priority:1"`
	CompanionID uint64 `gorm:"not null;uniqueIndex:uk_user_companion,priority:2;index"`
	Stage       string `gorm:"size:24;not null"`
	Narrative   string `gorm:"type:text;not null"`
	Milestones  string `gorm:"type:json;not null"`
	Revision    uint64 `gorm:"not null"`
}
