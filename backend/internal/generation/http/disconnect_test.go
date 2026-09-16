// disconnect_test.go 验证首帧断连和响应写入失败不会留下待执行任务。
package generationhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-chat/backend/internal/generation/domain"
)

// TestStartDisconnect 验证请求已取消时使用独立上下文收尾。
func TestStartDisconnect(t *testing.T) {
	tasks := &httpTasks{}
	engine := makeEngine(tasks, httpProvider{err: errors.New("provider must not run")})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/chats/3/generations",
		strings.NewReader(`{"model":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(httptest.NewRecorder(), req)
	if tasks.item.Status != domain.StatusCancelled {
		t.Fatalf("status=%s", tasks.item.Status)
	}
}

// brokenWriter 模拟连接已断开但请求上下文尚未收到取消通知。
type brokenWriter struct{ *frameRecorder }

// Write 将底层传输错误反馈给 SSE 编码器。
func (w brokenWriter) Write([]byte) (int, error) {
	return 0, errors.New("connection closed")
}

// WriteString 覆盖字符串快路径，避免测试误写入内存响应体。
func (w brokenWriter) WriteString(string) (int, error) {
	return 0, errors.New("connection closed")
}

// TestStartWriteError 验证底层写入失败同样终止待执行任务。
func TestStartWriteError(t *testing.T) {
	tasks := &httpTasks{}
	engine := makeEngine(tasks, httpProvider{err: errors.New("provider must not run")})
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/chats/3/generations",
		strings.NewReader(`{"model":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(brokenWriter{newFrameRecorder()}, req)
	if tasks.item.Status != domain.StatusCancelled {
		t.Fatalf("status=%s", tasks.item.Status)
	}
}
