// handler.go 提供角色列表和详情接口。
package characterhttp

import (
	"net/http"
	"strconv"

	"ai-chat/backend/internal/character/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 保存角色查询依赖。
type Handler struct {
	repo   domain.Repo
	logger *zap.Logger
}

// New 创建角色 Handler。
func New(repo domain.Repo, logger *zap.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

// Routes 注册受保护的角色查询路由。
func (h *Handler) Routes(engine *gin.Engine, auth gin.HandlerFunc) {
	engine.GET("/api/characters", auth, h.list)
	engine.GET("/api/characters/:id", auth, h.find)
	engine.GET("/api/v1/characters", auth, h.list)
	engine.GET("/api/v1/characters/:id", auth, h.find)
}

// list 返回当前用户的角色分页。
func (h *Handler) list(c *gin.Context) {
	userID, ok := contextUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	page, size := pageArgs(c)
	items, total, err := h.repo.List(c.Request.Context(), userID, page, size)
	if err != nil {
		h.logger.Error("character list failed", zap.Uint64("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "size": size})
}

// find 返回当前用户可见的角色详情。
func (h *Handler) find(c *gin.Context) {
	userID, ok := contextUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	item, err := h.repo.Find(c.Request.Context(), userID, id)
	if err != nil {
		h.logger.Warn("character not found", zap.Uint64("user_id", userID), zap.Uint64("character_id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"character": item})
}

// contextUser 读取鉴权中间件提供的用户编号。
func contextUser(c *gin.Context) (uint64, bool) {
	value, ok := c.Get("user_id")
	id, valid := value.(uint64)
	return id, ok && valid
}

// pageArgs 读取并限制分页参数，避免异常请求放大查询。
func pageArgs(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
