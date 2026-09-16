// dependency_test.go 验证 Redis 和 MySQL 驱动日志不会输出凭据或原始服务器错误。
package logx

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestDependencyLogs 验证各驱动适配器仅输出分类和可检索的事件标识。
func TestDependencyLogs(t *testing.T) {
	core, records := observer.New(zapcore.DebugLevel)
	log := zap.New(core)
	redis := NewRedis(log)
	redis.Printf(t.Context(), "private-template %s %v", "private-password", errors.New("private-response"))
	redis.Printf(t.Context(), "private-template %s %v", "changed-password", errors.New("changed-response"))
	NewMySQL(log).Print("private-dsn", errors.New("private-query"))
	entries := records.All()
	if len(entries) != 3 {
		t.Fatalf("entries=%+v", entries)
	}
	if entries[0].ContextMap()["event_id"] != entries[1].ContextMap()["event_id"] {
		t.Fatal("same template must have stable event id")
	}
	for _, entry := range entries {
		if entry.ContextMap()["component"] == nil || entry.ContextMap()["error"] == nil {
			t.Fatalf("missing context: %+v", entry)
		}
	}
	checkLogs(t, records, "private", "changed")
}

// TestSafeError 验证取消和超时仍可识别，同时未知错误原文不可见。
func TestSafeError(t *testing.T) {
	if SafeError(nil) != nil {
		t.Fatal("nil error must remain nil")
	}
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		if !errors.Is(SafeError(cause), cause) {
			t.Fatalf("classification lost: %v", cause)
		}
	}
	if SafeError(errors.New("private")).Error() != "dependency error (*errors.errorString)" {
		t.Fatal("unknown error should expose only its type")
	}
}
