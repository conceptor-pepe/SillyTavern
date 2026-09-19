// memory.go 定义记忆与模型能力契约，不依赖持久化框架。
package domain

import (
	"context"
	"errors"
)

// ErrInvalid 表示记忆输入不满足约束。
var ErrInvalid = errors.New("invalid memory")

// ErrNotFound 隐藏不存在和跨账号记录的差别。
var ErrNotFound = errors.New("memory not found")

// ErrBudget 表示当前输入无法在配置的上下文预算内保留。
var ErrBudget = errors.New("context budget exceeded")

// ErrCandidateConflict 表示候选已经被处理。
var ErrCandidateConflict = errors.New("memory candidate conflict")

// Entry 是用户主动维护的跨会话事实或关键词触发的世界书条目。
type Entry struct {
	ID          uint64    `json:"id,string"`
	UserID      uint64    `json:"-"`
	ChatID      uint64    `json:"-"`
	CharacterID uint64    `json:"-"`
	Kind        string    `json:"kind"`
	Content     string    `json:"content"`
	Keywords    []string  `json:"keywords"`
	Enabled     bool      `json:"enabled"`
	Pinned      bool      `json:"pinned"`
	Vector      []float64 `json:"-"`
	VectorModel string    `json:"-"`
}

// Summary 记录覆盖范围与内容摘要，不把历史分支当成全局事实。
type Summary struct {
	UserID, ChatID, EndID uint64
	Digest, Content       string
}

// Repo 所有读取和写入必须按用户与角色范围过滤。
type Repo interface {
	List(ctx context.Context, uid, charID uint64) ([]Entry, error)
	Save(ctx context.Context, item Entry) (Entry, error)
	Delete(ctx context.Context, uid, charID, id uint64) error
	Summaries(ctx context.Context, uid, chatID uint64) ([]Summary, error)
	SaveSummary(ctx context.Context, item Summary) error
}

// Summarizer 将已确认的历史前缀压缩成有界文本。
type Summarizer interface {
	Summarize(ctx context.Context, previous, transcript string) (string, error)
}

// Embedder 提供同一向量空间内的批量编码。
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
	Model() string
}

// ChatRepo 为独立故事会话提供记忆操作，首版不自动跨会话共享。
type ChatRepo interface {
	ListChat(ctx context.Context, uid, chatID uint64) ([]Entry, error)
	DeleteChat(ctx context.Context, uid, chatID, id uint64) error
}

// RelationshipRepo 为已建立关系的用户开放跨故事记忆，不要求用户拥有角色素材。
type RelationshipRepo interface {
	ListRelationship(ctx context.Context, uid, companionID uint64) ([]Entry, error)
	SaveRelationship(ctx context.Context, item Entry) (Entry, error)
	DeleteRelationship(ctx context.Context, uid, companionID, id uint64) error
}
