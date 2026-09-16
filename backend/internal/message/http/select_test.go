// select_test.go 验证候选选择路由的参数、错误和前端消息响应。
package messagehttp

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-chat/backend/internal/message/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// selectionRepo 只模拟原子选择能力，记录请求参数是否透传。
type selectionRepo struct {
	domain.Repo
	err    error
	called bool
}

// SelectVariant 验证调用范围并返回已保存的同父节点回复。
func (r *selectionRepo) SelectVariant(_ context.Context, uid, messageID, variantID uint64) (domain.Message, error) {
	r.called = true
	if uid != 7 || messageID != 8 || variantID != 21 {
		return domain.Message{}, domain.ErrNotFound
	}
	parent := uint64(3)
	return domain.Message{ID: 12, ConversationID: 2, ParentID: &parent, SourceVariantID: &variantID,
		Role: "assistant", Status: "completed", Content: "selected", VariantNo: 1}, r.err
}

// selectionRoute 使用真实应用层和 Handler 注册选择入口。
func selectionRoute(repo *selectionRepo) *gin.Engine {
	engine := gin.New()
	New(repo, nil, zap.NewNop()).Routes(engine, func(c *gin.Context) { c.Set("user_id", uint64(7)) })
	return engine
}

// TestSelectVariant 验证新消息及来源编号按字符串交付前端。
func TestSelectVariant(t *testing.T) {
	repo := &selectionRepo{}
	rec := httptest.NewRecorder()
	selectionRoute(repo).ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/messages/8/variants/21/select", nil))
	body := rec.Body.String()
	if rec.Code != 200 || !repo.called || !strings.Contains(body, `"id":"12"`) ||
		!strings.Contains(body, `"source_variant_id":"21"`) || !strings.Contains(body, `"parent_id":"3"`) {
		t.Fatalf("status=%d body=%s called=%v", rec.Code, body, repo.called)
	}
}

// TestSelectErrors 验证错误码和异常屏蔽，无效参数不得调用仓储。
func TestSelectErrors(t *testing.T) {
	for _, tc := range []struct {
		id     string
		cause  error
		status int
		called bool
	}{
		{"0", nil, 400, false}, {"bad", nil, 400, false},
		{"22", nil, 404, true}, {"21", domain.ErrNotFound, 404, true},
		{"21", domain.ErrConflict, 409, true}, {"21", errors.New("private database failure"), 500, true},
	} {
		repo := &selectionRepo{err: tc.cause}
		rec := httptest.NewRecorder()
		selectionRoute(repo).ServeHTTP(rec, httptest.NewRequest("POST",
			"/api/v1/messages/8/variants/"+tc.id+"/select", nil))
		if rec.Code != tc.status || repo.called != tc.called || strings.Contains(rec.Body.String(), "private") {
			t.Fatalf("status=%d body=%s called=%v", rec.Code, rec.Body.String(), repo.called)
		}
	}
}
