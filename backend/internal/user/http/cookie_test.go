// cookie_test.go 验证登录与退出使用相同的 Cookie 安全边界。
package userhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestCookie 检查默认安全属性、本地例外和删除属性。
func TestCookie(t *testing.T) {
	for _, local := range []bool{false, true} {
		for _, age := range []int{86400, -1} {
			h := NewLogin(nil, "test", zap.NewNop())
			if local {
				h.UseLocalHTTP()
			}
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			h.setCookie(ctx, "test", age)
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatal("expected one cookie")
			}
			c := cookies[0]
			if c.Secure == local || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				t.Fatalf("unexpected security attributes: local=%v", local)
			}
			if c.Path != "/" || c.MaxAge != age {
				t.Fatal("unexpected path or expiry")
			}
		}
	}
}
