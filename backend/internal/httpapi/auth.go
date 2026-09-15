// auth.go 提供 Gin 请求的用户身份认证中间件。
package httpapi

import (
	"net/http"
	"strings"
	"time"

	"ai-chat/backend/internal/reply"
	"ai-chat/backend/internal/user/auth"
	"github.com/gin-gonic/gin"
)

const userKey = "user_id"

// RequireAuth 拒绝没有有效用户令牌的请求。
func RequireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := readToken(c)
		token, err := auth.Parse(secret, value, time.Now())
		if err != nil || secret == "" || token.UserID == 0 {
			reply.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}
		c.Set(userKey, token.UserID)
		c.Set("user_version", token.Version)
		c.Next()
	}
}

// UserID 读取鉴权中间件写入的用户编号。
func UserID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(userKey)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}

// readToken 按 Cookie 优先、Bearer 其次读取会话令牌。
func readToken(c *gin.Context) string {
	if value, err := c.Cookie("ai_chat_token"); err == nil && value != "" {
		return value
	}
	header := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(header, prefix))
	}
	return ""
}
