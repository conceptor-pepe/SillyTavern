// memory_candidate.go 保存等待用户确认的记忆候选。
package model

// MemoryCandidate 记录模型提议及其处理状态，不会直接进入 Prompt。
type MemoryCandidate struct {
	Base
	UserID      uint64 `gorm:"not null;index:idx_candidate_chat,priority:1;uniqueIndex:uk_candidate_fingerprint,priority:1"`
	ChatID      uint64 `gorm:"not null;index:idx_candidate_chat,priority:2;uniqueIndex:uk_candidate_fingerprint,priority:2"`
	CompanionID uint64 `gorm:"not null;default:0;index"`
	SourceEndID uint64 `gorm:"not null"`
	Scope       string `gorm:"size:16;not null"`
	Content     string `gorm:"type:text;not null"`
	Evidence    string `gorm:"type:text;not null"`
	Fingerprint string `gorm:"size:64;not null;uniqueIndex:uk_candidate_fingerprint,priority:3"`
	Status      string `gorm:"size:16;not null;index"`
	MemoryID    *uint64
}
