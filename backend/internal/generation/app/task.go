// task.go 提供生成任务创建和状态变更用例。
package app

import (
	"context"
	"errors"
	"time"

	"ai-chat/backend/internal/generation/domain"
)

// Service 负责生成任务的业务规则。
type Service struct{ repo domain.Repo }

// New 创建生成任务服务。
func New(repo domain.Repo) *Service { return &Service{repo: repo} }

// Create 创建一个待执行的生成任务。
func (s *Service) Create(ctx context.Context, item domain.Generation) (domain.Generation, error) {
	if item.UserID == 0 || item.ConversationID == 0 {
		return domain.Generation{}, errors.New("owner and conversation are required")
	}
	if item.Provider == "" || item.Model == "" {
		return domain.Generation{}, errors.New("provider and model are required")
	}
	item.Status = domain.StatusPending
	return s.repo.Create(ctx, item)
}

// Find 查询当前用户的生成任务状态。
func (s *Service) Find(ctx context.Context, userID, id uint64) (domain.Generation, error) {
	if userID == 0 || id == 0 {
		return domain.Generation{}, domain.ErrNotFound
	}
	return s.repo.Find(ctx, userID, id)
}

// Cancel 将未结束任务标记为已取消。
func (s *Service) Cancel(ctx context.Context, userID, id uint64) error {
	return s.repo.Move(ctx, userID, id, []string{domain.StatusPending, domain.StatusRunning}, domain.Generation{
		Status: domain.StatusCancelled, FinishedAt: ptr(time.Now().Unix()),
	})
}

// CancelChat 由会话删除事务调用，取消全部未结束任务但不覆盖终态。
func (s *Service) CancelChat(ctx context.Context, userID, chatID uint64) error {
	if userID == 0 || chatID == 0 {
		return domain.ErrNotFound
	}
	return s.repo.CancelChat(ctx, userID, chatID, time.Now().Unix())
}

// Expire 清理超过时限仍在等待或运行的任务，不改写已有终态。
func (s *Service) Expire(ctx context.Context, before int64) (int64, error) {
	return s.repo.Expire(ctx, before)
}

// ptr 保留任务时间字段的可空语义，区分未发生和零值。
func ptr(value int64) *int64 { return &value }

// Start 将待执行任务置为运行中。
func (s *Service) Start(ctx context.Context, userID, id uint64, started int64) error {
	return s.repo.Move(ctx, userID, id, []string{domain.StatusPending}, domain.Generation{
		Status: domain.StatusRunning, StartedAt: &started,
	})
}

// Finish 将任务置为完成并关联 AI 消息。
func (s *Service) Finish(ctx context.Context, userID, id, messageID uint64, finished int64) error {
	return s.repo.Move(ctx, userID, id, []string{domain.StatusRunning}, domain.Generation{
		Status: domain.StatusCompleted, MessageID: &messageID, FinishedAt: &finished,
	})
}

// Fail 将任务置为失败并记录可检索错误。
func (s *Service) Fail(ctx context.Context, args FailArgs) error {
	return s.repo.Move(ctx, args.UserID, args.ID, []string{domain.StatusPending, domain.StatusRunning}, domain.Generation{
		Status: domain.StatusFailed, ErrorCode: args.Code, ErrorMessage: args.Message, FinishedAt: &args.Finished,
	})
}

// FailArgs 保存生成失败的更新参数。
type FailArgs struct {
	UserID   uint64
	ID       uint64
	Code     string
	Message  string
	Finished int64
}
