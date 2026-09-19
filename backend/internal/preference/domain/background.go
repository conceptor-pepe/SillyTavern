// background.go 定义账号级背景读写契约。
package domain

import "context"

// Background 不暴露数据库编号和用户归属。
type Background struct {
	Image   string `json:"image"`
	Enabled bool   `json:"enabled"`
}

// Repo 按账号隔离配置，空图片表示使用角色封面。
type Repo interface {
	Get(ctx context.Context, uid uint64) (Background, error)
	Save(ctx context.Context, uid uint64, item Background) error
}
