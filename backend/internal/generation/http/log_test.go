// log_test.go 验证生成执行失败日志保留定位字段但不泄漏上游或数据库原始内容。
package generationhttp

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	genapp "ai-chat/backend/internal/generation/app"
	"ai-chat/backend/internal/generation/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestRunLog 验证执行边界统一脱敏，包括 Runner 合并后的错误链。
func TestRunLog(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	tasks := genapp.New(&httpTasks{})
	runner := genapp.NewRunner(tasks, httpProvider{err: errors.New("private-token-and-prompt")}, httpMsgs{})
	handler := New(tasks, runner, Deps{}, zap.New(core))
	rec := newFrameRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("POST", "/", nil)
	ctx.Set("request_id", "test-request")
	handler.run(ctx, runArgs{uid: 7, chatID: 3, task: domain.Generation{ID: 8}, model: "test"})
	entries := logs.FilterMessage("generation run failed").All()
	if len(entries) != 1 {
		t.Fatalf("failure log count=%d", len(entries))
	}
	fields := entries[0].ContextMap()
	text, ok := fields["error"].(string)
	if !ok || text == "" || strings.Contains(text, "private-token-and-prompt") {
		t.Fatalf("unsafe error field: %v", fields)
	}
	if fields["user_id"] != uint64(7) || fields["chat_id"] != uint64(3) ||
		fields["generation_id"] != uint64(8) || fields["request_id"] != "test-request" {
		t.Fatalf("missing diagnostic fields: %v", fields)
	}
	if strings.Contains(rec.Body.String(), "private-token-and-prompt") ||
		!strings.Contains(rec.Body.String(), "event:generation_error") {
		t.Fatalf("unsafe failure response: %s", rec.Body.String())
	}
}
