// cleanup_mysql_test.go 验证未配置 Provider 的服务仍在启动时补偿遗留任务。
package httpapi

import (
	"context"
	"testing"
	"time"

	"ai-chat/backend/internal/config"
	appdb "ai-chat/backend/internal/db"
	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestCleanupStartup 验证生产装配不依赖 Provider 开关，停止后清理和连接均已释放。
func TestCleanupStartup(t *testing.T) {
	conn := testdb.Open(t)
	if err := appdb.Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	task := model.Generation{Base: model.Base{CreatedAt: time.Now().Add(-2 * domain.RunLimit)},
		UserID: 7, ConversationID: 3, Provider: "test", Model: "test", Status: domain.StatusPending}
	if err := conn.WithContext(t.Context()).Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	server, err := New(config.Config{MySQLDSN: conn.Dialector.(*mysql.Dialector).DSN}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	waitExpiry(t, conn, task.ID)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-server.cleaner.done:
	default:
		t.Fatal("Stop returned while cleanup still running")
	}
	pool, err := server.sql.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.PingContext(t.Context()); err == nil {
		t.Fatal("Stop left database connection open")
	}
}

// waitExpiry 有界轮询持久化事实，不依赖调度睡眠或日志代替数据库断言。
func waitExpiry(t *testing.T, conn *gorm.DB, id uint64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var task model.Generation
		if err := conn.WithContext(ctx).First(&task, id).Error; err != nil {
			t.Fatal(err)
		}
		if task.Status == domain.StatusFailed && task.ErrorCode == "generation_timeout" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("startup cleanup did not persist timeout")
		case <-ticker.C:
		}
	}
}
