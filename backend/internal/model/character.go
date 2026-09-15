// character.go 定义角色资料和角色资源的持久化模型。
package model

// Character 保存角色基础资料和 Prompt 相关字段。
type Character struct {
	Base
	UserID        uint64 `gorm:"not null;index"`
	Name          string `gorm:"size:128;not null"`
	Description   string `gorm:"type:text;not null"`
	Personality   string `gorm:"type:text;not null"`
	Scenario      string `gorm:"type:text;not null"`
	FirstMessage  string `gorm:"type:text;not null"`
	MessageSample string `gorm:"type:text;not null"`
	Creator       string `gorm:"size:128;not null"`
	CreatorNotes  string `gorm:"type:text;not null"`
	Tags          string `gorm:"type:json;not null"`
	ExtraData     string `gorm:"type:json;not null"`
}
