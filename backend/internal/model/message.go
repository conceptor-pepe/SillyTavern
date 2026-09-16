// message.go 定义消息、候选回复和生成任务模型。
package model

// Message 保存会话中的一条正式消息。
type Message struct {
	Base
	ConversationID  uint64  `gorm:"not null;index"`
	ParentID        *uint64 `gorm:"index"`
	SourceVariantID *uint64 `gorm:"uniqueIndex"`
	Role            string  `gorm:"size:32;not null;index"`
	Content         string  `gorm:"type:longtext;not null"`
	Status          string  `gorm:"size:32;not null;index"`
	VariantNo       int     `gorm:"not null;default:0"`
	ExtraData       string  `gorm:"type:json;not null"`
}

// MessageVariant 保存同一消息位置的候选回复。
type MessageVariant struct {
	Base
	MessageID uint64 `gorm:"not null;index"`
	VariantNo int    `gorm:"not null"`
	Content   string `gorm:"type:longtext;not null"`
	ExtraData string `gorm:"type:json;not null"`
}

// Generation 保存一次模型生成任务及其供应商标识。
type Generation struct {
	Base
	UserID         uint64  `gorm:"not null;index"`
	ConversationID uint64  `gorm:"not null;index"`
	MessageID      *uint64 `gorm:"index"`
	ProviderTaskID string  `gorm:"size:128;index"`
	Provider       string  `gorm:"size:64;not null"`
	Model          string  `gorm:"size:128;not null"`
	Status         string  `gorm:"size:32;not null;index"`
	ErrorCode      string  `gorm:"size:64"`
	ErrorMessage   string  `gorm:"size:512"`
	StartedAt      *int64
	FinishedAt     *int64
}
