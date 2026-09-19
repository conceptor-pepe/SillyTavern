// write.go 负责用户消息创建用例，确保消息写入前完成会话归属校验。
package app

import (
	"context"
	"errors"
	"strings"

	"ai-chat/backend/internal/message/domain"
	"go.uber.org/zap"
)

// Write 保存消息写入依赖。
type Write struct {
	messages domain.Repo
	mutator  domain.Mutator
	reviser  domain.AssistantReviser
	chats    domain.ChatRepo
	logger   *zap.Logger
}

// NewWrite 创建消息写入用例。
func NewWrite(messages domain.Repo, chats domain.ChatRepo, loggers ...*zap.Logger) *Write {
	mutator, _ := messages.(domain.Mutator)
	reviser, _ := messages.(domain.AssistantReviser)
	logger := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return &Write{messages: messages, mutator: mutator, reviser: reviser, chats: chats, logger: logger}
}

// ReviseAssistant 创建 AI 回复的编辑分支，原回复及其后续历史保持不变。
func (w *Write) ReviseAssistant(ctx context.Context, uid, messageID uint64, content string) (domain.Message, error) {
	content = strings.TrimSpace(content)
	if w.reviser == nil || uid == 0 || messageID == 0 || content == "" {
		return domain.Message{}, ErrQuery
	}
	return w.reviser.ReviseAssistant(ctx, uid, messageID, content)
}

// Edit 修改当前用户的用户消息，AI 消息和空内容不能被编辑。
func (w *Write) Edit(ctx context.Context, uid, messageID uint64, content string) (domain.Message, error) {
	if w.mutator == nil || uid == 0 || messageID == 0 || strings.TrimSpace(content) == "" {
		return domain.Message{}, ErrQuery
	}
	return w.mutator.Update(ctx, uid, messageID, strings.TrimSpace(content))
}

// Delete 删除当前用户可见的消息，底层使用软删除保留审计事实。
func (w *Write) Delete(ctx context.Context, uid, messageID uint64) error {
	if w.mutator == nil || uid == 0 || messageID == 0 {
		return ErrQuery
	}
	return w.mutator.Delete(ctx, uid, messageID)
}

// Create 保存一条当前用户消息，父消息用于保留分支关系。
func (w *Write) Create(ctx context.Context, uid, chatID uint64, item domain.Message) (domain.Message, error) {
	if uid == 0 || chatID == 0 || strings.TrimSpace(item.Content) == "" {
		return domain.Message{}, ErrQuery
	}
	if item.Role == "" {
		item.Role = "user"
	}
	if item.Role != "user" || item.ConversationID != chatID {
		return domain.Message{}, errors.New("invalid message role")
	}
	if item.ParentID != nil {
		checker, ok := w.messages.(domain.ParentChecker)
		if !ok {
			return domain.Message{}, errors.New("parent checker unavailable")
		}
		owned, err := checker.OwnsInChat(ctx, uid, chatID, *item.ParentID)
		if err != nil {
			w.logger.Error("message parent query failed", zap.Uint64("user_id", uid),
				zap.Uint64("chat_id", chatID), zap.Uint64("parent_id", *item.ParentID), zap.Error(err))
			return domain.Message{}, err
		}
		if !owned {
			return domain.Message{}, ErrQuery
		}
	}
	owned, err := w.chats.Owns(ctx, uid, chatID)
	if err != nil {
		w.logger.Error("message chat query failed", zap.Uint64("user_id", uid),
			zap.Uint64("chat_id", chatID), zap.Error(err))
		return domain.Message{}, err
	}
	if !owned {
		return domain.Message{}, ErrQuery
	}
	item.Status = "completed"
	return w.messages.Create(ctx, uid, item)
}
