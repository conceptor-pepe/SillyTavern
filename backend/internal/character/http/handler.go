// handler.go 提供角色列表和详情接口。
package characterhttp

import (
	"net/http"
	"strconv"

	"ai-chat/backend/internal/character/app"
	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 保存角色查询依赖。
type Handler struct {
	repo   domain.Repo
	create *app.Creator
	logger *zap.Logger
}

// New 创建角色 Handler。
func New(repo domain.Repo, logger *zap.Logger) *Handler {
	creator, _ := repo.(domain.CreatorRepo)
	return &Handler{repo: repo, create: app.NewCreator(creator), logger: logger}
}

// Routes 注册受保护的角色查询路由。
func (h *Handler) Routes(engine *gin.Engine, auth gin.HandlerFunc) {
	engine.POST("/api/characters", auth, h.createChar)
	engine.GET("/api/characters", auth, h.list)
	engine.GET("/api/characters/:id", auth, h.find)
	engine.POST("/api/v1/characters", auth, h.createChar)
	engine.GET("/api/v1/characters", auth, h.list)
	engine.GET("/api/v1/characters/:id", auth, h.find)
}

// createChar 创建当前用户拥有的角色。
func (h *Handler) createChar(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var in struct {
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Personality   string   `json:"personality"`
		Scenario      string   `json:"scenario"`
		FirstMessage  string   `json:"first_message"`
		Portrait      string   `json:"portrait"`
		Tags          []string `json:"tags"`
		Gender        string   `json:"gender"`
		Age           string   `json:"age"`
		MessageSample string   `json:"message_sample"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		h.logger.Warn("character input invalid", zap.Uint64("user_id", contextID(c)), zap.Error(err))
		reply.Fail(c, http.StatusBadRequest, "INVALID_INPUT", "角色资料无效")
		return
	}
	item, err := h.create.Create(c.Request.Context(), domain.Character{
		UserID: contextID(c), Name: in.Name, Description: in.Description,
		Personality: in.Personality, Scenario: in.Scenario, FirstMessage: in.FirstMessage,
		Portrait: in.Portrait, Tags: in.Tags, Gender: in.Gender, Age: in.Age, MessageSample: in.MessageSample,
	})
	if err != nil {
		h.logger.Warn("character create failed", zap.Uint64("user_id", contextID(c)), zap.Error(err))
		reply.Fail(c, http.StatusBadRequest, "INVALID_INPUT", "角色资料无效")
		return
	}
	h.logger.Info("character created", zap.Uint64("user_id", item.UserID), zap.Uint64("character_id", item.ID))
	reply.OK(c, item)
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

// contextID 读取鉴权中间件提供的用户编号。
func contextID(c *gin.Context) uint64 {
	value, _ := c.Get("user_id")
	id, _ := value.(uint64)
	return id
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
		h.logger.Warn("character id invalid", zap.Uint64("user_id", userID), zap.String("character_id", c.Param("id")))
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
