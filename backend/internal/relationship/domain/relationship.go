// relationship.go 定义跨故事共享的角色关系契约。
package domain

import (
	"context"
	"errors"
)

var (
	// ErrInvalid 表示关系资料不满足约束。
	ErrInvalid = errors.New("invalid relationship")
	// ErrNotFound 隐藏无关系、无会话和跨账号访问的差别。
	ErrNotFound = errors.New("relationship not found")
	// ErrConflict 表示客户端基于过期修订提交。
	ErrConflict = errors.New("relationship revision conflict")
)

// Relationship 是用户与一个稳定角色身份的共享关系档案。
type Relationship struct {
	ID          uint64   `json:"id,string"`
	UserID      uint64   `json:"-"`
	CompanionID uint64   `json:"companion_id,string"`
	Stage       string   `json:"stage"`
	Narrative   string   `json:"narrative"`
	Milestones  []string `json:"milestones"`
	Revision    uint64   `json:"revision,string"`
}

// Update 是带乐观锁的关系资料更新命令。
type Update struct {
	UserID     uint64
	ChatID     uint64
	Stage      string
	Narrative  string
	Milestones []string
	Revision   uint64
}

// Repo 提供按会话解析关系和带修订更新的最小能力。
type Repo interface {
	ByChat(context.Context, uint64, uint64) (Relationship, error)
	UpdateByChat(context.Context, Update) (Relationship, error)
}
