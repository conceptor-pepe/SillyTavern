// branch_test.go 验证生成与重新生成都使用保存的用户父链而非完整会话历史。
package generationhttp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	genapp "ai-chat/backend/internal/generation/app"
	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// branchMessages 将兄弟回复只放在普通历史中，便于发现错误的上下文入口。
type branchMessages struct {
	httpMessages
	missing bool
}

// Find 返回用于重新生成的旧回复，不应被追加到新上下文。
func (r branchMessages) Find(context.Context, uint64, uint64) (msgdomain.Message, error) {
	parent := uint64(2)
	item := msgdomain.Message{ID: 9, ConversationID: 3, Role: "assistant", ParentID: &parent}
	if r.missing {
		item.ParentID = nil
	}
	return item, nil
}

// List 返回不可出现在指定分支中的兄弟文本。
func (branchMessages) List(context.Context, uint64, uint64, int, int) ([]msgdomain.Message, int64, error) {
	return []msgdomain.Message{{Role: "assistant", Content: "sibling reply"}}, 1, nil
}

// Branch 按用户与会话校验叶节点，仅返回目标问题。
func (branchMessages) Branch(_ context.Context, uid, chatID, leafID uint64) ([]msgdomain.Message, error) {
	if uid != 7 || chatID != 3 || leafID != 2 {
		return nil, msgdomain.ErrNotFound
	}
	return []msgdomain.Message{{
		ID: 2, ConversationID: 3, Role: "user", Content: "question", Status: "completed",
	}}, nil
}

// branchProvider 记录真正送至供应商的模型上下文。
type branchProvider struct{ messages []provider.Message }

// Stream 模拟单候选完成，并保留输入供断言。
func (p *branchProvider) Stream(_ context.Context, req provider.Request) (provider.Stream, error) {
	p.messages = req.Messages
	return &httpStream{events: []provider.Event{{Type: "delta", Text: "answer"}, {Type: "done"}}}, nil
}

// branchEngine 注册生产 Handler，测试身份与生成依赖通过现有接口注入。
func branchEngine(p *branchProvider, repo branchMessages, tasks *httpTasks) *gin.Engine {
	service := genapp.New(tasks)
	engine := gin.New()
	New(service, genapp.NewRunner(service, p, httpMsgs{}), Deps{
		Chats: httpChats{owned: true}, Chars: httpChars{}, Msgs: repo,
	}, zap.NewNop()).Routes(engine, func(c *gin.Context) { c.Set("user_id", uint64(7)) })
	return engine
}

// TestBranchPrompt 验证父消息出现一次且兄弟回复不进入 Provider。
func TestBranchPrompt(t *testing.T) {
	for _, path := range []string{"/api/v1/chats/3/generations", "/api/v1/messages/9/regenerate"} {
		p := &branchProvider{}
		engine := branchEngine(p, branchMessages{}, &httpTasks{})
		rec := newFrameRecorder()
		req := httptest.NewRequest("POST", path,
			strings.NewReader(`{"model":"demo","parent_id":"2","content":"question"}`))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)
		if rec.Code != 200 || len(p.messages) != 2 || p.messages[1].Content != "question" {
			t.Fatalf("path=%s status=%d prompt=%v body=%s", path, rec.Code, p.messages, rec.Body.String())
		}
	}
}

// TestBranchRejected 验证无效父节点在创建任务及调用 Provider 前被拒绝。
func TestBranchRejected(t *testing.T) {
	for _, tc := range []struct {
		path, body string
		missing    bool
	}{
		{"/api/v1/chats/3/generations", `{"model":"demo","parent_id":"8"}`, false},
		{"/api/v1/chats/3/generations", `{"model":"demo","parent_id":"2","content":"changed"}`, false},
		{"/api/v1/messages/9/regenerate", `{"model":"demo"}`, true},
	} {
		p, tasks := &branchProvider{}, &httpTasks{}
		engine := branchEngine(p, branchMessages{missing: tc.missing}, tasks)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(rec, req)
		if rec.Code != 400 || p.messages != nil || tasks.item.ID != 0 {
			t.Fatalf("status=%d prompt=%v task=%+v", rec.Code, p.messages, tasks.item)
		}
	}
}
