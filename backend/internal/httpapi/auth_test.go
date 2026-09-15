// auth_test.go 验证 Gin 鉴权中间件的身份注入和拒绝行为。
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ai-chat/backend/internal/user/auth"
	"github.com/gin-gonic/gin"
)

// TestAuth 验证有效令牌能够进入受保护处理器。
func TestAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequireAuth("secret"))
	engine.GET("/", func(c *gin.Context) {
		id, ok := UserID(c)
		c.JSON(http.StatusOK, gin.H{"id": id, "ok": ok})
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Authorization", "Bearer "+auth.Sign("secret", 9, 1, time.Now()))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("got status %d", recorder.Code)
	}
}
