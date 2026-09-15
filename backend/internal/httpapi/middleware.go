// middleware.go 负责 HTTP 请求级别的通用中间件。
package httpapi

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestID 为每个请求生成或保留可追踪的请求编号。
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}
