// recovery.go 将请求异常交给 Zap，不输出 Cookie、请求体或 panic 原始值。
package httpapi

import (
	"errors"
	"net/http"

	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// recoverRequest 只记录稳定路由、请求标识和调用栈，避免异常携带聊天内容。
func recoverRequest(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() == nil {
				return
			}
			log.Error("http request panicked", zap.String("request_id", c.GetString("request_id")),
				zap.String("route", c.FullPath()), zap.Error(errors.New("request panic")),
				zap.Stack("stack"))
			c.Abort()
			if c.Writer.Written() {
				return
			}
			reply.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal error")
		}()
		c.Next()
	}
}
