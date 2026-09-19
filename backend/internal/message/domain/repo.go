// repo.go 定义消息域的查询和写入能力。
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrNotFound 隐藏不存在、已删除和其他用户消息之间的差异。
var ErrNotFound = errors.New("message not found")

// ErrConflict 表示消息当前状态不允许候选选择。
var ErrConflict = errors.New("message state conflict")

// Message 表示会话中的正式消息及其分支位置。
type Message struct {
	CreatedAt       time.Time       `json:"created_at"`
	ID              uint64          `json:"id,string"`
	ConversationID  uint64          `json:"conversation_id,string"`
	ParentID        *uint64         `json:"parent_id,string"`
	SourceVariantID *uint64         `json:"source_variant_id,string,omitempty"`
	Role            string          `json:"role"`
	Content         string          `json:"content"`
	Status          string          `json:"status"`
	VariantNo       int             `json:"variant_no"`
	ExtraData       json.RawMessage `json:"extra_data,omitempty"`
	Variants        []Variant       `json:"variants,omitempty"`
}

// Variant 表示同一消息位置的一条候选回复。
type Variant struct {
	ID        uint64          `json:"id,string"`
	MessageID uint64          `json:"message_id,string"`
	VariantNo int             `json:"variant_no"`
	Content   string          `json:"content"`
	ExtraData json.RawMessage `json:"extra_data,omitempty"`
}

// ChatRepo 提供会话归属校验。
type ChatRepo interface {
	Owns(ctx context.Context, userID, chatID uint64) (bool, error)
}

// Repo 提供带会话范围的消息访问能力。
type Repo interface {
	List(ctx context.Context, userID, chatID uint64, page, size int) ([]Message, int64, error)
	Create(ctx context.Context, userID uint64, item Message) (Message, error)
}

// VariantRepo 提供候选回复的持久化能力。
type VariantRepo interface {
	ListVariants(ctx context.Context, userID, messageID uint64, page, size int) ([]Variant, int64, error)
	CreateVariant(ctx context.Context, userID uint64, item Variant) (Variant, error)
}

// VariantSelector 在事务中校验候选归属并创建可重复读取的同父节点回复。
type VariantSelector interface {
	SelectVariant(ctx context.Context, userID, messageID, variantID uint64) (Message, error)
}

// Finder 提供按用户范围读取单条消息的能力。
type Finder interface {
	Find(ctx context.Context, userID, messageID uint64) (Message, error)
}

// BranchReader 返回指定消息及最近祖先，按祖先到叶节点排序且最多一百条。
type BranchReader interface {
	Branch(ctx context.Context, userID, chatID, messageID uint64) ([]Message, error)
}

// Mutator 提供受用户归属保护的消息修改能力。
type Mutator interface {
	Update(ctx context.Context, userID, messageID uint64, content string) (Message, error)
	Delete(ctx context.Context, userID, messageID uint64) error
}

// AssistantReviser 从已有 AI 回复创建同位置的新分支，不覆盖原消息。
type AssistantReviser interface {
	ReviseAssistant(ctx context.Context, userID, messageID uint64, content string) (Message, error)
}

// ParentChecker 校验父消息是否属于指定用户会话。
type ParentChecker interface {
	OwnsInChat(ctx context.Context, userID, chatID, messageID uint64) (bool, error)
}
