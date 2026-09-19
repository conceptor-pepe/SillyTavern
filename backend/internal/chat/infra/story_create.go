// story_create.go 故事版本和玩家快照在初始化事务中通过归属与状态校验。
package infra

import (
	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/model"
	persona "ai-chat/backend/internal/persona/domain"
	story "ai-chat/backend/internal/story/domain"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func createStory(tx *gorm.DB, in domain.StoryStart) (model.StorySession, error) {
	v, p, err := startSources(tx, in)
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return model.StorySession{}, err
	}
	var def story.Definition
	if err := json.Unmarshal([]byte(v.Definition), &def); err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return model.StorySession{}, err
	}
	if err := story.Validate(def); err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return model.StorySession{}, err
	}
	snapshot, err := json.Marshal(persona.Persona{ID: p.ID, Revision: p.Revision, Name: p.Name, Description: p.Description, Avatar: p.Avatar})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return model.StorySession{}, err
	}
	chat := model.Conversation{UserID: in.UserID, Mode: "story", Title: def.Title, Status: "active", ExtraData: "{}"}
	if err := tx.Create(&chat).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return model.StorySession{}, err
	}
	opening, err := saveOpening(tx, chat.ID, def)
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return model.StorySession{}, err
	}
	companionID, err := ensureCompanion(tx, in.UserID, v.UserID, def)
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return model.StorySession{}, err
	}
	session := model.StorySession{UserID: in.UserID, ChatID: chat.ID, VersionID: v.ID, CompanionID: companionID,
		PersonaID: p.ID, Persona: string(snapshot), StartKey: in.Key, RequestHash: startHash(in), OpeningID: opening}
	return session, tx.Create(&session).Error
}

func ensureCompanion(tx *gorm.DB, uid, authorID uint64, def story.Definition) (uint64, error) {
	id := def.Cast[0].CompanionID
	if id == 0 {
		return 0, nil
	}
	var companion model.Character
	if err := tx.Unscoped().Where("id = ? AND user_id = ?", id, authorID).First(&companion).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return 0, storyError(err)
	}
	relation := model.Relationship{UserID: uid, CompanionID: id, Stage: "acquaintance", Narrative: "", Milestones: "[]", Revision: 1}
	if err := tx.Where("user_id = ? AND companion_id = ?", uid, id).FirstOrCreate(&relation).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return 0, err
	}
	return id, nil
}

func startSources(tx *gorm.DB, in domain.StoryStart) (model.StoryVersion, model.PlayerPersona, error) {
	var v model.StoryVersion
	var p model.PlayerPersona
	if err := tx.Where("id = ?", in.VersionID).First(&v).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return v, p, storyError(err)
	}
	var s model.Story
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL", v.StoryID).First(&s).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return v, p, storyError(err)
	}
	if s.UserID != in.UserID && (s.PublishedVersionID == nil || *s.PublishedVersionID != v.ID) {
		return v, p, domain.ErrNotFound
	}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", in.PersonaID, in.UserID).First(&p).Error
	return v, p, storyError(err)
}

func saveOpening(tx *gorm.DB, chatID uint64, def story.Definition) (*uint64, error) {
	if len(def.Opening) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(struct {
		Schema   int             `json:"schema_version"`
		Segments []story.Segment `json:"segments"`
	}{1, def.Opening})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	row := model.Message{ConversationID: chatID, Role: "assistant", Status: "completed", Content: story.OpeningText(def), ExtraData: string(data)}
	if err := tx.Create(&row).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	return &row.ID, nil
}
