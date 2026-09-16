// variants_test.go 验证候选历史接口的分页响应和公开错误边界。
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

// historyRepo 仅实现候选查询所需能力，其他嵌入接口不得被调用。
type historyRepo struct {
	domain.Repo
	domain.VariantRepo
	err error
}

// Find 模拟用户范围查询，不暴露不可见消息的差异。
func (r *historyRepo) Find(_ context.Context, uid, id uint64) (domain.Message, error) {
	if r.err != nil {
		return domain.Message{}, r.err
	}
	if uid != 7 || id != 12 {
		return domain.Message{}, domain.ErrNotFound
	}
	return domain.Message{ID: id}, nil
}

// ListVariants 返回空页或稳定的候选记录，验证字符串 ID 编码。
func (r *historyRepo) ListVariants(_ context.Context, _, id uint64, page, size int) ([]domain.Variant, int64, error) {
	if page > 1 {
		return []domain.Variant{}, 1, nil
	}
	return []domain.Variant{{ID: 21, MessageID: id, VariantNo: 0, Content: "reply"}}, 1, nil
}

// historyRoute 使用测试身份中间件注册真实 Handler 路由。
func historyRoute(repo *historyRepo) *gin.Engine {
	engine := gin.New()
	New(repo, nil, zap.NewNop()).Routes(engine, func(c *gin.Context) {
		c.Set("user_id", uint64(7))
	})
	return engine
}

// TestVariantHistory 验证成功、默认分页和空页形状。
func TestVariantHistory(t *testing.T) {
	engine := historyRoute(&historyRepo{})
	for _, tc := range []struct {
		query, expected string
	}{
		{"", `"id":"21","message_id":"12"`},
		{"?page=2&size=1", `"items":[],"total":"1","page":2,"size":1`},
	} {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/messages/12/variants"+tc.query, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.expected) {
			t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
		}
	}
}

// TestVariantErrors 验证无效参数、不可见消息和内部错误不泄露敏感信息。
func TestVariantErrors(t *testing.T) {
	for _, tc := range []struct {
		path   string
		cause  error
		status int
		code   string
	}{
		{"12/variants?page=bad", nil, 400, "INVALID_QUERY"},
		{"12/variants?size=101", nil, 400, "INVALID_QUERY"},
		{"0/variants", nil, 400, "INVALID_QUERY"},
		{"other/variants", nil, 400, "INVALID_QUERY"},
		{"13/variants", nil, 404, "MESSAGE_NOT_FOUND"},
		{"12/variants", domain.ErrNotFound, 404, "MESSAGE_NOT_FOUND"},
		{"12/variants", errors.New("private database failure"), 500, "INTERNAL_ERROR"},
	} {
		rec := httptest.NewRecorder()
		historyRoute(&historyRepo{err: tc.cause}).ServeHTTP(rec,
			httptest.NewRequest("GET", "/api/v1/messages/"+tc.path, nil))
		body := rec.Body.String()
		if rec.Code != tc.status || !strings.Contains(body, tc.code) || strings.Contains(body, "private") {
			t.Fatalf("path=%s code=%d body=%s", tc.path, rec.Code, body)
		}
	}
}
