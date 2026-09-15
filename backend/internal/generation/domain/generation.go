// generation.go 定义 AI 生成任务实体、状态和仓储能力。
package domain

import (
	"context"
	"errors"
)

// 生成任务状态。
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// ErrNotFound 表示生成任务不存在或不属于当前用户。
var ErrNotFound = errors.New("generation not found")

// Generation 表示一次模型生成及其供应商标识。
type Generation struct {
	ID             uint64
	UserID         uint64
	ConversationID uint64
	MessageID      *uint64
	ProviderTaskID string
	Provider       string
	Model          string
	Status         string
	ErrorCode      string
	ErrorMessage   string
	StartedAt      *int64
	FinishedAt     *int64
}

// Repo 提供生成任务的持久化能力。
type Repo interface {
	Create(ctx context.Context, item Generation) (Generation, error)
	Find(ctx context.Context, userID, id uint64) (Generation, error)
	Update(ctx context.Context, userID, id uint64, patch Generation) error
	Move(ctx context.Context, userID, id uint64, from []string, patch Generation) error
	Expire(ctx context.Context, before int64) (int64, error)
}
