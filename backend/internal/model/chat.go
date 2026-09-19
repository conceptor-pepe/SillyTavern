// chat.go 定义会话和收藏关系的持久化模型。
package model

// Conversation 保存用户和角色之间的一次聊天会话。
type Conversation struct {
	Base
	Mode        string `gorm:"size:16;not null;default:legacy;index"`
	UserID      uint64 `gorm:"not null;index"`
	CharacterID uint64 `gorm:"not null;index"`
	Title       string `gorm:"size:255;not null"`
	Status      string `gorm:"size:32;not null;index"`
	LastMsgAt   *int64 `gorm:"index"`
	ExtraData   string `gorm:"type:json;not null"`
}

// Favorite 保存用户对角色或会话的收藏关系。
type Favorite struct {
	Base
	UserID      uint64 `gorm:"not null;index;uniqueIndex:uk_favorites_user_char_kind,priority:1"`
	CharacterID uint64 `gorm:"not null;uniqueIndex:uk_favorites_user_char_kind,priority:2"`
	Kind        string `gorm:"size:32;not null;uniqueIndex:uk_favorites_user_char_kind,priority:3"`
}
