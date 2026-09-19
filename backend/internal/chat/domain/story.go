// story.go 定义故事会话输入与可恢复的开场快照。
package domain

import (
	persona "ai-chat/backend/internal/persona/domain"
	relationship "ai-chat/backend/internal/relationship/domain"
	story "ai-chat/backend/internal/story/domain"
	"errors"
)

var ErrStoryInput = errors.New("invalid story session")
var ErrStoryConflict = errors.New("story session key conflict")

// StoryStart 只接收作者可见版本和本人人设，键用于重试幂等。
type StoryStart struct {
	UserID    uint64 `json:"-"`
	VersionID uint64 `json:"story_version_id,string"`
	PersonaID uint64 `json:"persona_id,string"`
	Key       string `json:"idempotency_key"`
}

// StoryState 返回当前用户的作品与玩家快照以及初始化消息编号。
type StoryState struct {
	Chat         Conversation               `json:"chat"`
	Version      story.Version              `json:"version"`
	Persona      persona.Persona            `json:"persona"`
	Relationship *relationship.Relationship `json:"relationship,omitempty"`
	OpeningID    *uint64                    `json:"opening_id,string"`
}
