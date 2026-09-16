// write.go 负责会话创建和删除用例，集中维护用户所有权规则。
package app

import (
	"context"
	"errors"
	"strings"

	"ai-chat/backend/internal/chat/domain"
)

// CharRepo 提供角色归属校验，避免会话绑定其他用户角色。
type CharRepo interface {
	Owns(ctx context.Context, userID, id uint64) (bool, error)
}

// Write 保存会话写入依赖。
type Write struct {
	chats  domain.Repo
	chars  CharRepo
	remove Deleter
}

// NewWrite 创建会话写入用例。
func NewWrite(chats domain.Repo, chars CharRepo, remove Deleter) *Write {
	return &Write{chats: chats, chars: chars, remove: remove}
}

// Create 创建属于当前用户的会话。
func (w *Write) Create(ctx context.Context, item domain.Conversation) (domain.Conversation, error) {
	if item.UserID == 0 || item.CharacterID == 0 || strings.TrimSpace(item.Title) == "" {
		return domain.Conversation{}, errors.New("invalid conversation")
	}
	owned, err := w.chars.Owns(ctx, item.UserID, item.CharacterID)
	if err != nil {
		return domain.Conversation{}, err
	}
	if !owned {
		return domain.Conversation{}, domain.ErrNotFound
	}
	if item.Status == "" {
		item.Status = "active"
	}
	return w.chats.Create(ctx, item)
}

// Delete 删除当前用户拥有的会话。
func (w *Write) Delete(ctx context.Context, userID, id uint64) error {
	if userID == 0 || id == 0 {
		return errors.New("invalid conversation")
	}
	return w.remove.Delete(ctx, userID, id)
}

// Rename 修改当前用户会话标题。
func (w *Write) Rename(ctx context.Context, userID, id uint64, title string) error {
	if userID == 0 || id == 0 || strings.TrimSpace(title) == "" {
		return errors.New("invalid conversation")
	}
	return w.chats.UpdateTitle(ctx, userID, id, strings.TrimSpace(title))
}
