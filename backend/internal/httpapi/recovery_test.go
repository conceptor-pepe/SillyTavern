// recovery_test.go 验证 HTTP 异常只产生安全的 Zap 日志和稳定错误响应。
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestRecoveryLogs 验证请求头、正文和 panic 原始值不会进入日志或响应。
func TestRecoveryLogs(t *testing.T) {
	core, records := observer.New(zapcore.DebugLevel)
	engine := gin.New()
	engine.Use(recoverRequest(zap.New(core)), requestID())
	engine.POST("/test", func(*gin.Context) { panic("private-panic") })
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("private-body"))
	req.Header.Set("Cookie", "ai_chat_token=private-cookie")
	req.Header.Set("Authorization", "Bearer private-token")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, req)
	entries := records.All()
	if response.Code != 500 || len(entries) != 1 {
		t.Fatalf("status=%d logs=%+v", response.Code, entries)
	}
	fields := entries[0].ContextMap()
	if fields["route"] != "/test" || fields["request_id"] == "" || fields["stack"] == nil {
		t.Fatalf("missing request context: %v", fields)
	}
	for _, value := range fields {
		text, ok := value.(string)
		if ok && strings.Contains(text, "private-") {
			t.Fatalf("sensitive log: %s", text)
		}
	}
	if strings.Contains(response.Body.String(), "private-") {
		t.Fatalf("sensitive response: %s", response.Body.String())
	}
}
