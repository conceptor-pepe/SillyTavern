// schema_lock_test.go 验证结构升级互斥、取消后释放及并发启动的真实 MySQL 行为。
package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"ai-chat/backend/internal/testdb"
	"gorm.io/gorm"
)

// TestSchemaConcurrent 两个连接池会话并发升级只能得到一条完成记录和正确索引。
func TestSchemaConcurrent(t *testing.T) {
	conn := testdb.Open(t)
	legacyFavorites(t, conn)
	start, results := make(chan struct{}), make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			results <- Migrate(t.Context(), conn)
		}()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	checkChange(t, conn, "applied")
	addFavorite(t, conn, 7, false)
	addFavorite(t, conn, 8, false)
	checkFavoriteRows(t, conn)
}

// TestSchemaCancelled 上下文取消后升级锁仍应释放，下一次升级不得永久阻塞。
func TestSchemaCancelled(t *testing.T) {
	conn := testdb.Open(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err := schemaLock(ctx, conn, func(*gorm.DB) error {
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	checkSchemaFree(t, conn)
	next, stop := context.WithTimeout(t.Context(), 2*time.Second)
	defer stop()
	if err := schemaLock(next, conn, func(*gorm.DB) error { return nil }); err != nil {
		t.Fatalf("lock retained: %v", err)
	}
}

// checkSchemaFree 检查服务端锁归属，避免同一连接递归加锁掩盖未释放问题。
func checkSchemaFree(t *testing.T, conn *gorm.DB) {
	t.Helper()
	var database string
	if err := conn.Raw("SELECT DATABASE()").Scan(&database).Error; err != nil {
		t.Fatal(err)
	}
	var owner sql.NullInt64
	if err := conn.Raw("SELECT IS_USED_LOCK(?)", schemaKey(database)).Scan(&owner).Error; err != nil || owner.Valid {
		t.Fatalf("schema lock leaked: %+v %v", owner, err)
	}
}

// TestSchemaWaiter 持锁时另一个升级超时不得进入回调，原始锁释放后可继续运行。
func TestSchemaWaiter(t *testing.T) {
	conn := testdb.Open(t)
	err := schemaLock(t.Context(), conn, func(*gorm.DB) error {
		wait, stop := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer stop()
		entered := false
		err := schemaLock(wait, conn, func(*gorm.DB) error {
			entered = true
			return nil
		})
		if err == nil || entered || wait.Err() != context.DeadlineExceeded {
			return errors.New("schema lock failed to serialize waiters")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	checkSchemaFree(t, conn)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
}
