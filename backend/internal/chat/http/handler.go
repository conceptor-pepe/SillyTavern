// handler.go 提供会话列表和详情接口。
package chathttp

import (
	"errors"
	"net/http"
	"strconv"

	"ai-chat/backend/internal/chat/app"
	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 保存会话查询依赖。
type Handler struct {
	story  *app.StoryService
	query  *app.Query
	recent *app.Recent
	write  *app.Write
	fav    *app.Favorite
	logger *zap.Logger
}

// New 创建会话 Handler。
func New(repo domain.Repo, chars app.CharRepo, remove app.Deleter, logger *zap.Logger, deps ...any) *Handler {
	var favorite app.FavoriteRepo
	var recent app.RecentRepo
	for _, dep := range deps {
		if value, ok := dep.(app.FavoriteRepo); ok {
			favorite = value
		}
		if value, ok := dep.(app.RecentRepo); ok {
			recent = value
		}
	}
	return &Handler{query: app.NewQuery(repo), recent: app.NewRecent(recent), write: app.NewWrite(repo, chars, remove), fav: app.NewFavorite(favorite), logger: logger}
}

// list 返回当前用户的会话分页。
func (h *Handler) list(c *gin.Context) {
	page, size, err := pageArgs(c)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	result, err := h.query.List(c.Request.Context(), userID(c), page, size)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	reply.OK(c, result)
}

// recentList 返回当前用户最近有消息的会话。
func (h *Handler) recentList(c *gin.Context) {
	page, size, err := pageArgs(c)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	result, err := h.recent.List(c.Request.Context(), userID(c), page, size)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	reply.OK(c, result)
}

// find 返回当前用户可见的会话详情。
func (h *Handler) find(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, app.ErrQuery)
		return
	}
	item, err := h.query.Find(c.Request.Context(), userID(c), id)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	reply.OK(c, item)
}

// create 创建当前用户与角色之间的新会话。
func (h *Handler) create(c *gin.Context) {
	var in struct {
		Mode        string `json:"mode"`
		VersionID   uint64 `json:"story_version_id,string"`
		PersonaID   uint64 `json:"persona_id,string"`
		Key         string `json:"idempotency_key"`
		CharacterID uint64 `json:"character_id"`
		Title       string `json:"title"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, app.ErrQuery)
		return
	}
	if in.Mode == "story" {
		if in.CharacterID != 0 {
			h.fail(c, app.ErrQuery)
			return
		}
		h.startStory(c, domain.StoryStart{UserID: userID(c), VersionID: in.VersionID, PersonaID: in.PersonaID, Key: in.Key})
		return
	}
	if (in.Mode != "" && in.Mode != "legacy") || in.VersionID != 0 || in.PersonaID != 0 || in.Key != "" {
		h.fail(c, app.ErrQuery)
		return
	}
	item, err := h.write.Create(c.Request.Context(), domain.Conversation{
		UserID: userID(c), CharacterID: in.CharacterID, Title: in.Title,
	})
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	h.logger.Info("chat created", zap.Uint64("user_id", item.UserID), zap.Uint64("chat_id", item.ID))
	reply.OK(c, item)
}

// remove 删除当前用户的会话。
func (h *Handler) remove(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, app.ErrQuery)
		return
	}
	if err := h.write.Delete(c.Request.Context(), userID(c), id); err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	h.logger.Info("chat deleted", zap.Uint64("user_id", userID(c)), zap.Uint64("chat_id", id))
	reply.OK(c, struct{}{})
}

// rename 修改当前用户会话标题。
func (h *Handler) rename(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, app.ErrQuery)
		return
	}
	var in struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, app.ErrQuery)
		return
	}
	if err := h.write.Rename(c.Request.Context(), userID(c), id, in.Title); err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	h.logger.Info("chat renamed", zap.Uint64("user_id", userID(c)), zap.Uint64("chat_id", id))
	reply.OK(c, struct{}{})
}

// favorite 设置当前用户的会话收藏状态。
func (h *Handler) favorite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, app.ErrQuery)
		return
	}
	on := c.Request.Method == http.MethodPut
	if err := h.fav.Set(c.Request.Context(), userID(c), id, on); err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		h.fail(c, err)
		return
	}
	h.logger.Info("chat favorite changed", zap.Uint64("user_id", userID(c)), zap.Uint64("chat_id", id), zap.Bool("favorite", on))
	reply.OK(c, struct{}{})
}

// userID 读取鉴权中间件提供的用户编号。
func userID(c *gin.Context) uint64 {
	value, _ := c.Get("user_id")
	id, _ := value.(uint64)
	return id
}

// pageArgs 读取并限制分页参数。
func pageArgs(c *gin.Context) (int, int, error) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return 0, 0, app.ErrQuery
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", "20"))
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return 0, 0, app.ErrQuery
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return 0, 0, app.ErrQuery
	}
	return page, size, nil
}

// fail 区分不可见对象与基础设施故障，禁止将数据库故障伪装成 404。
func (h *Handler) fail(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL_ERROR"
	switch {
	case errors.Is(err, app.ErrIdentity):
		status, code = http.StatusUnauthorized, "UNAUTHORIZED"
	case errors.Is(err, domain.ErrStoryConflict):
		status, code = http.StatusConflict, "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, app.ErrQuery), errors.Is(err, domain.ErrStoryInput):
		status, code = http.StatusBadRequest, "INVALID_QUERY"
	case errors.Is(err, domain.ErrNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	}
	fields := []zap.Field{zap.String("request_id", c.GetString("request_id")),
		zap.Uint64("user_id", userID(c)), zap.String("chat_id", c.Param("id")),
		zap.String("error_code", code), zap.Error(err)}
	if status == http.StatusInternalServerError {
		h.logger.Error("chat query failed", fields...)
	} else {
		h.logger.Warn("chat query rejected", fields...)
	}
	reply.Fail(c, status, code, http.StatusText(status))
}
