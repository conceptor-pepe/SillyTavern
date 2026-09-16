// chat_test.go 验证前置查询后会话被删除时，创建入口仍返回稳定的不可见错误。
package generationhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chatdomain "ai-chat/backend/internal/chat/domain"
)

// TestCreateChatGone 模拟前置归属检查成功但事务检查失败，不允许启动 SSE 或暴露内部错误。
func TestCreateChatGone(t *testing.T) {
	engine := makeEngine(&httpTasks{err: chatdomain.ErrNotFound}, httpProvider{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats/3/generations",
		strings.NewReader(`{"model":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"NOT_FOUND"`) ||
		strings.Contains(rec.Body.String(), "event:") {
		t.Fatalf("deleted chat response: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
