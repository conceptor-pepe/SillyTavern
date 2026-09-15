// repo.go 定义角色域的数据访问能力。
package domain

import "context"

// Character 表示可供用户使用的角色资料。
type Character struct {
	ID           uint64
	UserID       uint64
	Name         string
	Description  string
	Personality  string
	Scenario     string
	FirstMessage string
}

// Repo 提供带用户范围的角色查询。
type Repo interface {
	List(ctx context.Context, userID uint64, page, size int) ([]Character, int64, error)
	Find(ctx context.Context, userID, id uint64) (Character, error)
	Owns(ctx context.Context, userID, id uint64) (bool, error)
}
