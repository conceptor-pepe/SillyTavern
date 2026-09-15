// login_test.go 验证登录 Handler 的成功响应和令牌 Cookie。
package userhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-chat/backend/internal/user/app"
	"ai-chat/backend/internal/user/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type fakeRepo struct{}

// FindHandle 返回测试用户。
func (fakeRepo) FindHandle(_ context.Context, _ string) (domain.User, error) {
	hash, _ := app.HashPass("secret")
	return domain.User{ID: 3, Handle: "demo", Name: "Demo", PasswordHash: hash, Enabled: true}, nil
}

// FindID 返回测试用户。
func (fakeRepo) FindID(_ context.Context, _ uint64) (domain.User, error) {
	return fakeRepo{}.FindHandle(context.Background(), "")
}

// Create 返回带编号的测试用户。
func (fakeRepo) Create(_ context.Context, user domain.User) (domain.User, error) {
	user.ID, user.Version, user.Enabled = 4, 1, true
	return user, nil
}

// TestLogin 验证登录成功后返回用户并设置 Cookie。
func TestLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewLogin(fakeRepo{}, "secret", zap.NewNop()).RegisterRoutes(engine, func(c *gin.Context) {
		c.Set("user_id", uint64(3))
		c.Next()
	})
	req := httptest.NewRequest(http.MethodPost, "/api/users/login", strings.NewReader(`{"handle":"demo","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Set-Cookie") == "" {
		t.Fatalf("status=%d cookie=%q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
}
