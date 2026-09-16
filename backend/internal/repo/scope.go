// scope.go 在业务回调的上下文中携带事务，使各域只操作自己拥有的数据表。
package repo

import (
	"context"

	"gorm.io/gorm"
)

// scopeKey 是私有事务上下文键，避免与请求值发生冲突。
type scopeKey struct{}

// InTx 开启事务并把同一连接交给回调；已有事务时使用保存点保护回调原子性。
func InTx(ctx context.Context, db *gorm.DB, run func(context.Context) error) error {
	return DB(ctx, db).Transaction(func(tx *gorm.DB) error {
		return run(context.WithValue(ctx, scopeKey{}, tx))
	})
}

// DB 返回回调绑定的事务，未进入事务时使用调用者的连接；不能跨数据库混用上下文。
func DB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(scopeKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}
