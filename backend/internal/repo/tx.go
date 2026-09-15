// tx.go 提供统一的 GORM 事务执行入口。
package repo

import (
	"context"

	"gorm.io/gorm"
)

// WithTx 在一个事务中执行多个写操作，失败时自动回滚。
func WithTx(ctx context.Context, db *gorm.DB, fn func(*gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(fn)
}
