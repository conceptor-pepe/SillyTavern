// story.go 定义故事版本契约，定义内容按整份聚合校验和冻结。
package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid story")
	ErrNotFound = errors.New("story not found")
	ErrConflict = errors.New("story revision conflict")
)

// Cast 是故事内角色副本，ID 在该作品的版本间保持稳定。
type Cast struct {
	ID          string `json:"id"`
	CompanionID uint64 `json:"companion_id,string,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Personality string `json:"personality"`
	Gender      string `json:"gender"`
	Age         string `json:"age"`
	Portrait    string `json:"portrait"`
}

// Segment 区分旁白和角色发言，开场禁止替玩家发言。
type Segment struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	SpeakerID string `json:"speaker_id,omitempty"`
	Text      string `json:"text"`
}

// Lore 保存作品内的可触发设定，不引用作者私有记忆。
type Lore struct {
	Content  string   `json:"content"`
	Keywords []string `json:"keywords"`
	Pinned   bool     `json:"pinned"`
}

// Definition 是可版本化的作品聚合；首版只允许一个角色。
type Definition struct {
	SchemaVersion int       `json:"schema_version"`
	Title         string    `json:"title"`
	Hook          string    `json:"hook"`
	Cover         string    `json:"cover"`
	Tags          []string  `json:"tags"`
	World         string    `json:"world"`
	Cast          []Cast    `json:"cast"`
	Opening       []Segment `json:"opening"`
	Examples      string    `json:"examples"`
	Lore          []Lore    `json:"lore"`
}

// Story 是作者可编辑的草稿。
type Story struct {
	ID                 uint64     `json:"id,string"`
	UserID             uint64     `json:"-"`
	Revision           uint64     `json:"revision,string"`
	PublishedVersionID *uint64    `json:"published_version_id,string,omitempty"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
	Definition         Definition `json:"definition"`
}

// Version 是不可变的可玩快照，并不意味着已经公开。
type Version struct {
	ID         uint64     `json:"id,string"`
	StoryID    uint64     `json:"story_id,string"`
	Revision   uint64     `json:"revision,string"`
	Digest     string     `json:"digest"`
	Definition Definition `json:"definition"`
}

// PublicStory 是发现页可见的冻结作品，不暴露作者草稿。
type PublicStory struct {
	StoryID      uint64     `json:"story_id,string"`
	VersionID    uint64     `json:"version_id,string"`
	AuthorID     uint64     `json:"author_id,string"`
	AuthorName   string     `json:"author_name"`
	AuthorHandle string     `json:"author_handle"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	Definition   Definition `json:"definition"`
}
