// repo.go 通过本人会话解析稳定角色，客户端不能伪造角色归属。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/relationship/domain"
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 保存关系档案的数据连接。
type Repo struct{ db *gorm.DB }

// New 创建关系仓储。
// @param db 数据连接
// @return 关系仓储
func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// ByChat 从本人会话解析共享关系档案。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @return 关系档案与错误
func (r *Repo) ByChat(ctx context.Context, uid, chatID uint64) (domain.Relationship, error) {
	companionID, err := companionByChat(r.db.WithContext(ctx), uid, chatID)
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.Relationship{}, err
	}
	var row model.Relationship
	if err := r.db.WithContext(ctx).Where("user_id = ? AND companion_id = ? AND deleted_at IS NULL", uid, companionID).First(&row).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.Relationship{}, mapError(err)
	}
	return decode(row)
}

// UpdateByChat 锁定共享关系行并校验预期修订。
// @param ctx 请求上下文
// @param in 更新命令
// @return 更新后的关系与错误
func (r *Repo) UpdateByChat(ctx context.Context, in domain.Update) (domain.Relationship, error) {
	var row model.Relationship
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		companionID, err := companionByChat(tx, in.UserID, in.ChatID)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND companion_id = ? AND deleted_at IS NULL", in.UserID, companionID)
		if err := query.First(&row).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return mapError(err)
		}
		if row.Revision != in.Revision {
			return domain.ErrConflict
		}
		milestones, err := json.Marshal(in.Milestones)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		row.Stage, row.Narrative, row.Milestones, row.Revision = in.Stage, in.Narrative, string(milestones), row.Revision+1
		return tx.Model(&row).Select("stage", "narrative", "milestones", "revision").Updates(&row).Error
	})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.Relationship{}, err
	}
	return decode(row)
}

func companionByChat(db *gorm.DB, uid, chatID uint64) (uint64, error) {
	var chat model.Conversation
	if err := db.Where("id = ? AND user_id = ? AND deleted_at IS NULL", chatID, uid).First(&chat).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return 0, mapError(err)
	}
	if chat.Mode != "story" && chat.CharacterID != 0 {
		return chat.CharacterID, nil
	}
	var session model.StorySession
	if err := db.Where("chat_id = ? AND user_id = ?", chatID, uid).First(&session).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return 0, mapError(err)
	}
	if session.CompanionID == 0 {
		return 0, domain.ErrNotFound
	}
	return session.CompanionID, nil
}

func decode(row model.Relationship) (domain.Relationship, error) {
	out := domain.Relationship{ID: row.ID, UserID: row.UserID, CompanionID: row.CompanionID,
		Stage: row.Stage, Narrative: row.Narrative, Revision: row.Revision}
	err := json.Unmarshal([]byte(row.Milestones), &out.Milestones)
	return out, err
}

func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
