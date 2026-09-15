// repo.go 定义用户域需要的数据访问能力。
package domain

import "context"

// Repo 提供用户查询能力，具体存储由基础设施层实现。
type Repo interface {
	FindHandle(ctx context.Context, handle string) (User, error)
	FindID(ctx context.Context, id uint64) (User, error)
	Create(ctx context.Context, user User) (User, error)
}
