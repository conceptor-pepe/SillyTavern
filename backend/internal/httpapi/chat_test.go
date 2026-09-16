// chat_test.go 验证会话路由与真实鉴权中间件的组合，不替代 MySQL 集成验证。
package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-chat/backend/internal/chat/domain"
	chathttp "ai-chat/backend/internal/chat/http"
	"ai-chat/backend/internal/httpapi"
	"ai-chat/backend/internal/user/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// chatStore 记录查询范围，模拟存储返回而非实际 SQL 隔离。
type chatStore struct {
	uid   uint64
	page  int
	size  int
	err   error
	calls int
}

// Owns 返回测试会话归属结果。
func (s *chatStore) Owns(_ context.Context, uid, id uint64) (bool, error) {
	return uid == 7 && id == 3, s.err
}

// Create 返回测试会话。
func (s *chatStore) Create(_ context.Context, item domain.Conversation) (domain.Conversation, error) {
	item.ID = 3
	return item, s.err
}

// Delete 返回测试删除结果。
func (s *chatStore) Delete(_ context.Context, _, _ uint64) error {
	return s.err
}

// UpdateTitle 修改测试会话标题。
func (s *chatStore) UpdateTitle(_ context.Context, _, _ uint64, _ string) error {
	return s.err
}

// List 记录用户及分页参数并返回空列表。
func (s *chatStore) List(_ context.Context, uid uint64, page, size int) ([]domain.Conversation, int64, error) {
	s.uid, s.page, s.size = uid, page, size
	s.calls++
	return nil, 0, s.err
}

// Find 只有用户 7 的会话 3 可见，其他组合均模拟未找到。
func (s *chatStore) Find(_ context.Context, uid, id uint64) (domain.Conversation, error) {
	s.uid = uid
	s.calls++
	if s.err != nil {
		return domain.Conversation{}, s.err
	}
	if uid != 7 || id != 3 {
		return domain.Conversation{}, domain.ErrNotFound
	}
	return domain.Conversation{ID: 3, UserID: 7, Title: "demo"}, nil
}

// chatRequest 使用真实令牌和路由执行 HTTP 请求。
func chatRequest(store *chatStore, path string, uid uint64) *httptest.ResponseRecorder {
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("request_id", "chat-test")
		c.Next()
	})
	chathttp.New(store, fakeChars{}, store, zap.NewNop()).Routes(engine, httpapi.RequireAuth("test-secret"))
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if uid != 0 {
		req.Header.Set("Authorization", "Bearer "+auth.Sign("test-secret", uid, 1, time.Now()))
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

type fakeChars struct{}

// Owns 返回测试角色归属结果。
func (fakeChars) Owns(_ context.Context, uid, id uint64) (bool, error) {
	return uid == 7 && id == 11, nil
}

// TestChatStatus 验证未登录、跨用户、参数错误及数据库异常不会返回成功。
func TestChatStatus(t *testing.T) {
	tests := []struct {
		name, path    string
		uid           uint64
		err           error
		status, calls int
	}{
		{"missing auth", "/api/v1/chats", 0, nil, 401, 0},
		{"bad page", "/api/v1/chats?page=x", 7, nil, 400, 0},
		{"negative page", "/api/v1/chats?page=-1", 7, nil, 400, 0},
		{"huge page", "/api/v1/chats?page=1000001", 7, nil, 400, 0},
		{"huge size", "/api/v1/chats?size=101", 7, nil, 400, 0},
		{"zero id", "/api/v1/chats/0", 7, nil, 400, 0},
		{"invalid id", "/api/v1/chats/nope", 7, nil, 400, 0},
		{"other owner", "/api/v1/chats/3", 8, nil, 404, 1},
		{"missing", "/api/v1/chats/4", 7, nil, 404, 1},
		{"list fault", "/api/v1/chats", 7, errors.New("private db failure"), 500, 1},
		{"detail fault", "/api/v1/chats/3", 7, errors.New("private db failure"), 500, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &chatStore{err: tt.err}
			rec := chatRequest(store, tt.path, tt.uid)
			if rec.Code != tt.status || store.calls != tt.calls {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, store.calls, rec.Body)
			}
		})
	}
}

// TestChatList 验证分页透传、空数组及统一响应信封。
func TestChatList(t *testing.T) {
	store := &chatStore{}
	rec := chatRequest(store, "/api/v1/chats?page=2&size=5", 7)
	var body struct {
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
		Data      struct {
			Items []domain.Conversation `json:"items"`
			Page  int                   `json:"page"`
			Size  int                   `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || body.Code != "OK" || body.RequestID != "chat-test" || body.Data.Items == nil {
		t.Fatalf("invalid envelope: %s", rec.Body)
	}
	if store.uid != 7 || store.page != 2 || store.size != 5 || body.Data.Page != 2 || body.Data.Size != 5 {
		t.Fatalf("invalid pagination: %s", rec.Body)
	}
}

// TestChatDetail 验证详情使用下划线字段且不泄露所有者内部字段。
func TestChatDetail(t *testing.T) {
	rec := chatRequest(&chatStore{}, "/api/v1/chats/3", 7)
	var body struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || string(body.Data["id"]) != `"3"` || body.Data["character_id"] == nil {
		t.Fatalf("invalid detail: %s", rec.Body)
	}
	if body.Data["user_id"] != nil || body.Data["UserID"] != nil {
		t.Fatalf("owner leaked: %s", rec.Body)
	}
}

// TestChatCreate 验证创建会话会传递当前用户并返回统一响应。
func TestChatCreate(t *testing.T) {
	store := &chatStore{}
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("request_id", "chat-test")
		c.Next()
	})
	chathttp.New(store, fakeChars{}, store, zap.NewNop()).Routes(engine, httpapi.RequireAuth("test-secret"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chats",
		strings.NewReader(`{"character_id":11,"title":"new chat"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth.Sign("test-secret", 7, 1, time.Now()))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":"OK"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
}

// TestChatDelete 验证删除会话需要登录并调用软删除用例。
func TestChatDelete(t *testing.T) {
	store := &chatStore{}
	engine := gin.New()
	chathttp.New(store, fakeChars{}, store, zap.NewNop()).Routes(engine, httpapi.RequireAuth("test-secret"))
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/chats/3", nil)
	req.Header.Set("Authorization", "Bearer "+auth.Sign("test-secret", 7, 1, time.Now()))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
}
