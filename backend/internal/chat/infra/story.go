// story.go 在同一事务中认领开聊键、冻结玩家资料和初始化开场。
package infra

import (
	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/model"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StoryRepo 保存故事会话事务所需连接。
type StoryRepo struct{ db *gorm.DB }

// NewStory 创建故事会话仓储。
// @param db 数据连接
// @return 会话仓储
func NewStory(db *gorm.DB) *StoryRepo { return &StoryRepo{db: db} }

// Start 锁定用户行串行认领幂等键，不会将半初始化会话暴露给客户端。
// @param ctx 请求上下文
// @param in 开聊参数
// @return 会话快照与错误
func (r *StoryRepo) Start(ctx context.Context, in domain.StoryStart) (domain.StoryState, error) {
	var chatID uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL", in.UserID).First(&user).Error; err != nil {
			// audit:allow-no-log 应用层记录仓储错误。
			return storyError(err)
		}
		session, found, err := findStart(tx, in)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if found {
			chatID = session.ChatID
			return nil
		}
		session, err = createStory(tx, in)
		chatID = session.ChatID
		return err
	})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.StoryState{}, err
	}
	return r.State(ctx, in.UserID, chatID)
}

func findStart(tx *gorm.DB, in domain.StoryStart) (model.StorySession, bool, error) {
	var row model.StorySession
	err := tx.Where("user_id = ? AND start_key = ?", in.UserID, in.Key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return row, false, err
	}
	if row.RequestHash != startHash(in) {
		return row, false, domain.ErrStoryConflict
	}
	return row, true, nil
}

func startHash(in domain.StoryStart) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("v1:%d:%d", in.VersionID, in.PersonaID))))
}

func storyError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
