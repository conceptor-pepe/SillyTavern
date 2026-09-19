// memory.go 持久化用户可编辑记忆、世界书与可校验的分支摘要。
package model

// Memory 按用户和角色隔离，向量模型名称避免不同空间混用。
type Memory struct {
	Base
	UserID         uint64 `gorm:"not null;index:idx_memory_scope,priority:1;index:idx_memory_chat,priority:1"`
	ChatID         uint64 `gorm:"not null;default:0;index:idx_memory_chat,priority:2"`
	CharacterID    uint64 `gorm:"not null;index:idx_memory_scope,priority:2"`
	Kind           string `gorm:"size:16;not null"`
	Content        string `gorm:"type:text;not null"`
	Keywords       string `gorm:"type:json;not null"`
	Enabled        bool   `gorm:"not null"`
	Pinned         bool   `gorm:"not null"`
	Embedding      string `gorm:"type:json;not null"`
	EmbeddingModel string `gorm:"size:255;not null"`
}

// MemorySummary 缓存历史前缀摘要，摘要只能在源消息散列匹配时复用。
type MemorySummary struct {
	Base
	UserID  uint64 `gorm:"not null;uniqueIndex:uk_summary,priority:1"`
	ChatID  uint64 `gorm:"not null;uniqueIndex:uk_summary,priority:2"`
	EndID   uint64 `gorm:"not null;uniqueIndex:uk_summary,priority:3"`
	Digest  string `gorm:"size:64;not null"`
	Content string `gorm:"type:text;not null"`
}
