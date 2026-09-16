// default_test.go 验证消费者无需传递模型名即可生成回复。
package generationhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	provider "ai-chat/backend/internal/provider/domain"
)

// TestDefaultModel 验证省略模型时使用服务端配置并完成流式回复。
func TestDefaultModel(t *testing.T) {
	tasks := &httpTasks{}
	engine := makeEngine(tasks, httpProvider{stream: &httpStream{
		events: []provider.Event{{Type: "delta", Text: "hello"}, {Type: "done"}},
	}}, "server-default")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats/3/generations", strings.NewReader(`{"n":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := newFrameRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || tasks.item.Model != "server-default" || !strings.Contains(rec.Body.String(), "event:message_end") {
		t.Fatalf("status=%d model=%s", rec.Code, tasks.item.Model)
	}
}
