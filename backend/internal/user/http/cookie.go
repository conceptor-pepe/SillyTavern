// cookie.go 统一注册、登录和退出的 Cookie 属性。
package userhttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// UseLocalHTTP 由装配层在确认仅回环监听后启用，禁止依据请求头降级。
func (h *LoginHandler) UseLocalHTTP() {
	h.localHTTP = true
}

// setCookie 保留 HttpOnly 和同站限制，仅本地 HTTP 关闭 Secure。
func (h *LoginHandler) setCookie(c *gin.Context, token string, age int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("ai_chat_token", token, age, "/", "", !h.localHTTP, true)
}
