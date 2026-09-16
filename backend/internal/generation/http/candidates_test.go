// candidates_test.go 验证候选数量从 HTTP 传至 Provider，以及带索引的 SSE 结果。
package generationhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	genapp "ai-chat/backend/internal/generation/app"
	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type candidateProvider struct{ count int }

// Stream 记录请求数量并返回乱序候选，模拟真实供应商交错发送。
func (p *candidateProvider) Stream(_ context.Context, req provider.Request) (provider.Stream, error) {
	p.count = req.N
	return &httpStream{events: []provider.Event{
		{Type: "delta", Index: 1, Text: "second"},
		{Type: "delta", Index: 0, Text: "first"},
		{Type: "done"},
	}}, nil
}

type candidateWriter struct {
	httpMsgs
	item msgdomain.Message
}

// SaveDone 模拟事务边界，记录本次统一写入的消息和候选。
func (w *candidateWriter) SaveDone(_ context.Context, _, _ uint64, item msgdomain.Message, _ int64) (msgdomain.Message, error) {
	item.ID = 12
	w.item = item
	return item, nil
}

// TestCandidateStream 验证请求数量、候选索引和完成消息穿过完整 HTTP 编排。
func TestCandidateStream(t *testing.T) {
	p, writer := &candidateProvider{}, &candidateWriter{}
	tasks := genapp.New(&httpTasks{})
	engine := gin.New()
	New(tasks, genapp.NewRunner(tasks, p, writer), Deps{
		Chats: httpChats{owned: true}, Chars: httpChars{}, Msgs: httpMessages{},
	}, zap.NewNop()).Routes(engine, func(c *gin.Context) { c.Set("user_id", uint64(7)) })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats/3/generations",
		strings.NewReader(`{"model":"demo","n":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := newFrameRecorder()
	engine.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || p.count != 2 || len(writer.item.Variants) != 2 {
		t.Fatalf("status=%d count=%d item=%+v body=%s", rec.Code, p.count, writer.item, body)
	}
	if writer.item.Content != "first" || !strings.Contains(body, `"index":1`) ||
		!strings.Contains(body, `"variants":[`) || !strings.Contains(body, "event:message_end") {
		t.Fatalf("item=%+v body=%s", writer.item, body)
	}
}

// TestInvalidCount 验证两个入口都在创建任务前拒绝非法候选数量。
func TestInvalidCount(t *testing.T) {
	for _, path := range []string{"/api/v1/chats/3/generations", "/api/v1/messages/9/regenerate"} {
		for _, body := range []string{`{"model":"demo","n":-1}`, `{"model":"demo","n":5}`} {
			tasks := &httpTasks{}
			engine := makeEngine(tasks, httpProvider{})
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || tasks.item.ID != 0 {
				t.Fatalf("path=%s status=%d item=%+v", path, rec.Code, tasks.item)
			}
		}
	}
}
