// handler_test.go 验证生成 SSE 接口的鉴权、参数和事件输出。
package generationhttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chardomain "ai-chat/backend/internal/character/domain"
	chatdomain "ai-chat/backend/internal/chat/domain"
	genapp "ai-chat/backend/internal/generation/app"
	"ai-chat/backend/internal/generation/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type httpTasks struct {
	item domain.Generation
	err  error
}

type httpChats struct {
	owned bool
}

func (httpChats) List(context.Context, uint64, int, int) ([]chatdomain.Conversation, int64, error) {
	return nil, 0, nil
}
func (httpChats) Find(context.Context, uint64, uint64) (chatdomain.Conversation, error) {
	return chatdomain.Conversation{}, nil
}
func (r httpChats) Owns(context.Context, uint64, uint64) (bool, error) { return r.owned, nil }
func (httpChats) Create(context.Context, chatdomain.Conversation) (chatdomain.Conversation, error) {
	return chatdomain.Conversation{}, nil
}
func (httpChats) UpdateTitle(context.Context, uint64, uint64, string) error { return nil }
func (httpChats) Delete(context.Context, uint64, uint64) error              { return nil }

type httpChars struct{}

func (httpChars) List(context.Context, uint64, int, int) ([]chardomain.Character, int64, error) {
	return nil, 0, nil
}
func (httpChars) Find(context.Context, uint64, uint64) (chardomain.Character, error) {
	return chardomain.Character{Name: "角色"}, nil
}
func (httpChars) Owns(context.Context, uint64, uint64) (bool, error) { return true, nil }

type httpMessages struct{}

func (httpMessages) List(context.Context, uint64, uint64, int, int) ([]msgdomain.Message, int64, error) {
	return nil, 0, nil
}
func (httpMessages) Create(context.Context, uint64, msgdomain.Message) (msgdomain.Message, error) {
	return msgdomain.Message{}, nil
}

func (r *httpTasks) Create(_ context.Context, item domain.Generation) (domain.Generation, error) {
	item.ID = 8
	r.item = item
	return item, r.err
}
func (r *httpTasks) Find(context.Context, uint64, uint64) (domain.Generation, error) {
	return r.item, r.err
}
func (r *httpTasks) Update(context.Context, uint64, uint64, domain.Generation) error { return r.err }
func (r *httpTasks) Move(_ context.Context, _ uint64, _ uint64, _ []string, patch domain.Generation) error {
	r.item.Status = patch.Status
	return r.err
}
func (r *httpTasks) Expire(context.Context, int64) (int64, error) { return 0, r.err }

// CancelChat 提供删除编排所需的仓储能力，HTTP 生成测试不模拟批量持久化。
func (r *httpTasks) CancelChat(context.Context, uint64, uint64, int64) error { return r.err }

// TestFind 验证生成任务状态可以按当前用户查询。
func TestFind(t *testing.T) {
	tasks := &httpTasks{item: domain.Generation{ID: 8, UserID: 7, Status: domain.StatusRunning}}
	engine := makeEngine(tasks, httpProvider{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/generations/8", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"Status":"running"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// TestFindMissing 验证不存在的生成任务返回 404。
func TestFindMissing(t *testing.T) {
	tasks := &httpTasks{err: domain.ErrNotFound}
	engine := makeEngine(tasks, httpProvider{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/generations/8", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type httpStream struct {
	events []provider.Event
	index  int
}

func (s *httpStream) Next(context.Context) (provider.Event, error) {
	if s.index == len(s.events) {
		return provider.Event{}, errors.New("stream ended")
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}
func (s *httpStream) Close() error { return nil }

type httpProvider struct {
	stream provider.Stream
	err    error
}

func (p httpProvider) Stream(context.Context, provider.Request) (provider.Stream, error) {
	return p.stream, p.err
}

type httpMsgs struct{}

func (httpMsgs) Create(_ context.Context, _ uint64, item msgdomain.Message) (msgdomain.Message, error) {
	item.ID = 12
	return item, nil
}

// makeEngine 创建带固定测试身份的生成路由。
func makeEngine(tasks *httpTasks, p provider.Provider, models ...string) *gin.Engine {
	engine := gin.New()
	model := ""
	if len(models) > 0 {
		model = models[0]
	}
	runner := genapp.NewRunner(genapp.New(tasks), p, httpMsgs{})
	New(genapp.New(tasks), runner, Deps{DefaultModel: model, Chats: httpChats{owned: true}, Chars: httpChars{}, Msgs: httpMessages{}}, zap.NewNop()).Routes(engine, func(c *gin.Context) {
		c.Set("user_id", uint64(7))
		c.Next()
	})
	return engine
}

// TestStreamForbidden 验证不属于当前用户的会话不能创建生成任务。
func TestStreamForbidden(t *testing.T) {
	engine := gin.New()
	tasks := &httpTasks{}
	runner := genapp.NewRunner(genapp.New(tasks), httpProvider{}, httpMsgs{})
	New(genapp.New(tasks), runner, Deps{Chats: httpChats{owned: false}, Chars: httpChars{}, Msgs: httpMessages{}}, zap.NewNop()).Routes(engine, func(c *gin.Context) {
		c.Set("user_id", uint64(7))
		c.Next()
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats/3/generations", strings.NewReader(`{"model":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || tasks.item.ID != 0 {
		t.Fatalf("status=%d item=%+v", rec.Code, tasks.item)
	}
}

// TestStream 验证 SSE 成功事件按开始、增量、结束顺序输出。
func TestStream(t *testing.T) {
	engine := makeEngine(&httpTasks{}, httpProvider{stream: &httpStream{
		events: []provider.Event{{Type: "delta", Text: "hi"}, {Type: "done"}},
	}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats/3/generations", strings.NewReader(`{"model":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := newFrameRecorder()
	engine.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "event:message_start") ||
		!strings.Contains(body, "event:message_delta") || !strings.Contains(body, "event:message_end") {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
}

// TestStreamBadBody 验证缺少模型时不会创建生成任务。
func TestStreamBadBody(t *testing.T) {
	tasks := &httpTasks{}
	engine := makeEngine(tasks, httpProvider{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats/3/generations", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || tasks.item.ID != 0 {
		t.Fatalf("status=%d item=%+v", rec.Code, tasks.item)
	}
}

// TestRegenerateBadBody 验证重新生成必须明确传入模型。
func TestRegenerateBadBody(t *testing.T) {
	tasks := &httpTasks{}
	engine := makeEngine(tasks, httpProvider{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/9/regenerate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || tasks.item.ID != 0 {
		t.Fatalf("status=%d item=%+v", rec.Code, tasks.item)
	}
}

// TestLastUser 验证旧消息数组只取最后一条用户输入。
func TestLastUser(t *testing.T) {
	items := []provider.Message{
		{Role: "user", Content: "第一轮"},
		{Role: "assistant", Content: "回复"},
		{Role: "user", Content: "第二轮"},
	}
	if got := lastUser("", items); got != "第二轮" {
		t.Fatalf("content=%q", got)
	}
	if got := lastUser("新输入", items); got != "新输入" {
		t.Fatalf("content=%q", got)
	}
}
