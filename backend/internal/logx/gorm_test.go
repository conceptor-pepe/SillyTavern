// gorm_test.go 验证日志级别、并发配置副本和数据库敏感字段隔离。
package logx

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm/logger"
)

// TestGormTrace 验证错误与慢查询可追踪，但任何级别都不展开 SQL。
func TestGormTrace(t *testing.T) {
	core, records := observer.New(zapcore.DebugLevel)
	log := NewGorm(zap.New(core))
	sql := func() (string, int64) {
		t.Error("SQL callback must never be called")
		return "secret SQL", 1
	}
	cause := fmt.Errorf("private wrapper: %w", &mysql.MySQLError{Number: 1062, Message: "private-value"})
	log.Trace(t.Context(), time.Now(), sql, cause)
	log.Trace(t.Context(), time.Now().Add(-time.Second), sql, nil)
	log.Trace(t.Context(), time.Now(), sql, nil)
	log.Trace(t.Context(), time.Now().Add(-time.Second), sql, logger.ErrRecordNotFound)
	log.LogMode(logger.Silent).Trace(t.Context(), time.Now(), sql, cause)
	log.LogMode(logger.Info).Trace(t.Context(), time.Now(), sql, nil)
	entries := records.All()
	if len(entries) != 3 || entries[0].Level != zapcore.ErrorLevel ||
		entries[1].Level != zapcore.WarnLevel || entries[2].Level != zapcore.InfoLevel {
		t.Fatalf("entries=%+v", entries)
	}
	if entries[0].ContextMap()["error"] != "mysql error 1062" {
		t.Fatalf("error=%v", entries[0].ContextMap())
	}
	checkLogs(t, records, "private", "secret SQL")
}

// TestGormModes 验证 LogMode 不修改原实例，GORM 模板及参数不会落日志。
func TestGormModes(t *testing.T) {
	core, records := observer.New(zapcore.DebugLevel)
	base := NewGorm(zap.New(core))
	verbose := base.LogMode(logger.Info)
	verbose.Info(t.Context(), "secret-format", "secret-arg")
	base.Info(t.Context(), "must-not-log")
	base.Warn(t.Context(), "secret-format", "secret-arg")
	base.Error(t.Context(), "secret-format", fmt.Errorf("secret-error"))
	silent := base.LogMode(logger.Silent)
	silent.Info(t.Context(), "secret-format")
	silent.Warn(t.Context(), "secret-format")
	silent.Error(t.Context(), "secret-format")
	if records.Len() != 3 {
		t.Fatalf("entries=%+v", records.All())
	}
	checkLogs(t, records, "secret", "must-not-log")
}

// checkLogs 检查最终结构化字段，避免只验证日志消息而遗漏错误字段。
func checkLogs(t *testing.T, records *observer.ObservedLogs, secrets ...string) {
	t.Helper()
	for _, entry := range records.All() {
		fields, err := json.Marshal(entry.ContextMap())
		if err != nil {
			t.Fatal(err)
		}
		text := entry.Message + string(fields)
		checkSecrets(t, text, secrets)
	}
}

// checkSecrets 检查序列化日志中的每个敏感标记，保持循环层次清晰。
func checkSecrets(t *testing.T, text string, secrets []string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Fatalf("log leaked %q: %s", secret, text)
		}
	}
}
