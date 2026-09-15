// routes.go 定义会话 API 路径和统一鉴权边界。
package chathttp

import "github.com/gin-gonic/gin"

// Routes 注册受保护的 v1 会话查询路由。
func (h *Handler) Routes(engine *gin.Engine, auth gin.HandlerFunc) {
	engine.GET("/api/v1/chats", auth, h.list)
	engine.GET("/api/v1/chats/recent", auth, h.recentList)
	engine.GET("/api/v1/chats/:id", auth, h.find)
	engine.POST("/api/v1/chats", auth, h.create)
	engine.PATCH("/api/v1/chats/:id", auth, h.rename)
	engine.PUT("/api/v1/chats/:id/favorite", auth, h.favorite)
	engine.DELETE("/api/v1/chats/:id/favorite", auth, h.favorite)
	engine.DELETE("/api/v1/chats/:id", auth, h.remove)
}
