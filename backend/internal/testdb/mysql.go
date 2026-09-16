// mysql.go 为真实 MySQL 集成测试创建独立数据库，测试结束仅清理本次随机命名的库。
package testdb

import (
	"os"
	"strings"
	"testing"

	sqlmysql "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 按显式测试 DSN 创建隔离库，账号需要建库权限；未配置时明确跳过。
func Open(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("AI_CHAT_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("AI_CHAT_TEST_MYSQL_DSN is required for real MySQL verification")
	}
	config, err := sqlmysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.DBName = ""
	config.ParseTime = true
	admin := connect(t, config.FormatDSN())
	name := "ai_chat_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.WithContext(t.Context()).Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// 测试 Context 在 Cleanup 前会被取消，清理使用独立连接自身的上下文。
		if err := admin.Exec("DROP DATABASE `" + name + "`").Error; err != nil {
			t.Error(err)
		}
	})
	config.DBName = name
	return connect(t, config.FormatDSN())
}

// connect 为测试连接登记释放函数，避免测试失败时泄漏连接。
func connect(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}
