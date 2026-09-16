// repo.go 定义角色域的数据访问能力。
package domain

import (
	"context"
	"errors"
)

// ErrInvalid 表示角色资料不符合业务规则。
var ErrInvalid = errors.New("invalid character")

// Character 表示可供用户使用的角色资料。
type Character struct {
	ID            uint64
	UserID        uint64
	Name          string
	Description   string
	Personality   string
	Scenario      string
	FirstMessage  string
	Portrait      string
	Tags          []string
	Gender        string
	Age           string
	MessageSample string
}

// Repo 提供带用户范围的角色查询。
type Repo interface {
	List(ctx context.Context, userID uint64, page, size int) ([]Character, int64, error)
	Find(ctx context.Context, userID, id uint64) (Character, error)
	Owns(ctx context.Context, userID, id uint64) (bool, error)
}

// CreatorRepo 提供角色创建能力。
type CreatorRepo interface {
	Create(ctx context.Context, item Character) (Character, error)
}
