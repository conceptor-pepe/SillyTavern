// story_state.go 已开始会话读取冻结版本，不受素材或作品软删除影响。
package infra

import (
	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/model"
	relationship "ai-chat/backend/internal/relationship/domain"
	story "ai-chat/backend/internal/story/domain"
	"context"
	"encoding/json"
)

// State 以会话归属为入口读取不可变版本和玩家快照。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 会话编号
// @return 会话快照与错误
func (r *StoryRepo) State(ctx context.Context, uid, id uint64) (domain.StoryState, error) {
	out := domain.StoryState{}
	var chat model.Conversation
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ? AND mode = ? AND deleted_at IS NULL", id, uid, "story").First(&chat).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return out, storyError(err)
	}
	var session model.StorySession
	if err := r.db.WithContext(ctx).Where("chat_id = ? AND user_id = ?", id, uid).First(&session).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return out, storyError(err)
	}
	var version model.StoryVersion
	if err := r.db.WithContext(ctx).Where("id = ?", session.VersionID).First(&version).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return out, storyError(err)
	}
	out.Chat = toChat(chat)
	out.OpeningID = session.OpeningID
	out.Version = story.Version{ID: version.ID, StoryID: version.StoryID, Revision: version.Revision, Digest: version.Digest}
	if err := json.Unmarshal([]byte(version.Definition), &out.Version.Definition); err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return out, err
	}
	err := json.Unmarshal([]byte(session.Persona), &out.Persona)
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return out, err
	}
	if session.CompanionID == 0 {
		return out, nil
	}
	var relation model.Relationship
	if err := r.db.WithContext(ctx).Where("user_id = ? AND companion_id = ? AND deleted_at IS NULL", uid, session.CompanionID).First(&relation).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return out, storyError(err)
	}
	out.Relationship = &relationship.Relationship{ID: relation.ID, UserID: uid, CompanionID: relation.CompanionID,
		Stage: relation.Stage, Narrative: relation.Narrative, Revision: relation.Revision}
	return out, json.Unmarshal([]byte(relation.Milestones), &out.Relationship.Milestones)
}
