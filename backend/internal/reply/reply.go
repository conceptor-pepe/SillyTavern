// reply.go 统一普通 JSON 接口的响应信封，不用于 SSE 事件。
package reply

import "github.com/gin-gonic/gin"

// Envelope 定义普通 API 的稳定响应字段。
type Envelope[T any] struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	RequestID string `json:"request_id"`
}

// OK 写入带请求编号的成功结果。
func OK[T any](c *gin.Context, data T) {
	c.JSON(200, Envelope[T]{Code: "OK", Message: "ok", Data: data, RequestID: c.GetString("request_id")})
}

// Fail 返回公开错误描述，不泄漏内部数据库或供应商错误。
func Fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Envelope[*struct{}]{
		Code: code, Message: message, RequestID: c.GetString("request_id"),
	})
}
