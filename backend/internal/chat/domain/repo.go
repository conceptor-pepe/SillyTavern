// repo.go 定义会话域的数据访问能力。
package domain

import (
	"context"
	"errors"
)

// ErrNotFound 隐藏不存在及不属于当前用户的会话。
var ErrNotFound = errors.New("conversation not found")

// Conversation 表示用户与角色之间的一次聊天会话。
type Conversation struct {
	Mode        string `json:"mode"`
	ID          uint64 `json:"id,string"`
	UserID      uint64 `json:"-"`
	CharacterID uint64 `json:"character_id,string"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	LastMsgAt   *int64 `json:"last_msg_at,string"`
}

// Repo 提供带用户范围的会话查询。
type Repo interface {
	List(ctx context.Context, userID uint64, page, size int) ([]Conversation, int64, error)
	Find(ctx context.Context, userID, id uint64) (Conversation, error)
	Owns(ctx context.Context, userID, id uint64) (bool, error)
	Create(ctx context.Context, item Conversation) (Conversation, error)
	UpdateTitle(ctx context.Context, userID, id uint64, title string) error
	Delete(ctx context.Context, userID, id uint64) error
}
