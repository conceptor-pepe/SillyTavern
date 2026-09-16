// logger_mysql_test.go 使用真实 MySQL 验证连接级 GORM 日志和重复键错误脱敏。
package db

import (
	"strings"
	"testing"

	"ai-chat/backend/internal/testdb"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/driver/mysql"
)

// TestMySQLLogs 验证生产连接不会把重复键中的私密消息值写入 Zap。
func TestMySQLLogs(t *testing.T) {
	fixture := testdb.Open(t)
	core, records := observer.New(zapcore.DebugLevel)
	conn, err := OpenMySQL(t.Context(), fixture.Dialector.(*mysql.Dialector).DSN, zap.New(core))
	if err != nil {
		t.Fatal(err)
	}
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	conn = conn.WithContext(t.Context())
	if err := conn.Exec("CREATE TABLE log_secrets (value VARCHAR(128) UNIQUE)").Error; err != nil {
		t.Fatal(err)
	}
	const secret = "private-chat-and-password"
	if err := conn.Exec("INSERT INTO log_secrets VALUES (?)", secret).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec("INSERT INTO log_secrets VALUES (?)", secret).Error; err == nil {
		t.Fatal("duplicate key must fail")
	}
	entries := records.FilterMessage("database query failed").All()
	if len(entries) != 1 || entries[0].ContextMap()["error"] != "mysql error 1062" {
		t.Fatalf("entries=%+v", entries)
	}
	for _, entry := range records.All() {
		if strings.Contains(entry.Message, secret) || entry.ContextMap()["sql"] != nil {
			t.Fatalf("private SQL logged: %+v", entry)
		}
	}
}
