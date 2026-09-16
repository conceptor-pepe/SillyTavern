// handler.go 提供会话消息历史查询接口。
package messagehttp

import (
	"errors"
	"net/http"
	"strconv"

	"ai-chat/backend/internal/message/app"
	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 保存消息查询用例和日志依赖。
type Handler struct {
	query  *app.Query
	write  *app.Write
	logger *zap.Logger
}

// New 创建消息 Handler。
func New(messages app.MessageRepo, chats app.ChatRepo, logger *zap.Logger) *Handler {
	return &Handler{query: app.NewQuery(messages, chats), write: app.NewWrite(messages, chats), logger: logger}
}

// Routes 注册受保护的消息历史接口。
func (h *Handler) Routes(engine *gin.Engine, auth gin.HandlerFunc) {
	engine.GET("/api/v1/chats/:id/messages", auth, h.list)
	engine.POST("/api/v1/chats/:id/messages", auth, h.create)
	engine.PATCH("/api/v1/messages/:id", auth, h.edit)
	engine.DELETE("/api/v1/messages/:id", auth, h.delete)
	engine.GET("/api/v1/messages/:id/variants", auth, h.variants)
	engine.POST("/api/v1/messages/:id/variants/:variant/select", auth, h.selectVariant)
}

// edit 修改当前用户的一条消息内容。
func (h *Handler) edit(c *gin.Context) {
	uid, messageID, err := readMessage(c)
	if err != nil {
		h.fail(c, err)
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, app.ErrQuery)
		return
	}
	item, err := h.write.Edit(c.Request.Context(), uid, messageID, in.Content)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.logger.Info("message edited", zap.Uint64("user_id", uid), zap.Uint64("message_id", item.ID))
	reply.OK(c, item)
}

// delete 删除当前用户的一条消息，并保留数据库软删除记录。
func (h *Handler) delete(c *gin.Context) {
	uid, messageID, err := readMessage(c)
	if err != nil {
		h.fail(c, err)
		return
	}
	if err := h.write.Delete(c.Request.Context(), uid, messageID); err != nil {
		h.fail(c, err)
		return
	}
	h.logger.Info("message deleted", zap.Uint64("user_id", uid), zap.Uint64("message_id", messageID))
	reply.OK(c, gin.H{"deleted": true})
}

// create 保存当前用户发送的一条消息。
func (h *Handler) create(c *gin.Context) {
	uid, chatID, _, _, err := readArgs(c)
	if err != nil {
		h.fail(c, err)
		return
	}
	var in struct {
		Content  string  `json:"content"`
		ParentID *uint64 `json:"parent_id,string"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		h.fail(c, app.ErrQuery)
		return
	}
	item, err := h.write.Create(c.Request.Context(), uid, chatID, domain.Message{
		ConversationID: chatID, ParentID: in.ParentID, Content: in.Content,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	h.logger.Info("message created", zap.Uint64("user_id", uid), zap.Uint64("chat_id", chatID),
		zap.Uint64("message_id", item.ID))
	reply.OK(c, item)
}

// list 返回当前用户可见会话的消息历史。
func (h *Handler) list(c *gin.Context) {
	uid, chatID, page, size, err := readArgs(c)
	if err != nil {
		h.fail(c, err)
		return
	}
	items, total, err := h.query.List(c.Request.Context(), uid, chatID, page, size)
	if err != nil {
		h.fail(c, err)
		return
	}
	reply.OK(c, struct {
		Items []domain.Message `json:"items"`
		Total int64            `json:"total,string"`
		Page  int              `json:"page"`
		Size  int              `json:"size"`
	}{items, total, page, size})
}

// readArgs 读取用户身份、会话编号和分页参数。
func readArgs(c *gin.Context) (uint64, uint64, int, int, error) {
	value, _ := c.Get("user_id")
	uid, ok := value.(uint64)
	chatID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("size", "20"))
	if !ok || err != nil || pageErr != nil || sizeErr != nil {
		return 0, 0, 0, 0, app.ErrQuery
	}
	return uid, chatID, page, size, nil
}

// readMessage 读取受保护消息接口的用户和消息编号。
func readMessage(c *gin.Context) (uint64, uint64, error) {
	value, _ := c.Get("user_id")
	uid, ok := value.(uint64)
	messageID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if !ok || err != nil || uid == 0 || messageID == 0 {
		return 0, 0, app.ErrQuery
	}
	return uid, messageID, nil
}

// fail 屏蔽内部错误，只返回稳定的公开错误码。
func (h *Handler) fail(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "INTERNAL_ERROR"
	if errors.Is(err, app.ErrQuery) {
		status, code = http.StatusBadRequest, "INVALID_QUERY"
	}
	if errors.Is(err, domain.ErrNotFound) {
		status, code = http.StatusNotFound, "MESSAGE_NOT_FOUND"
	}
	if errors.Is(err, domain.ErrConflict) {
		status, code = http.StatusConflict, "MESSAGE_CONFLICT"
	}
	fields := []zap.Field{zap.String("request_id", c.GetString("request_id")),
		zap.Uint64("user_id", userValue(c)), zap.String("resource_id", c.Param("id")),
		zap.String("route", c.FullPath()), zap.String("error_code", code), zap.Error(err)}
	if status == http.StatusInternalServerError {
		h.logger.Error("message request failed", fields...)
	} else {
		h.logger.Warn("message request rejected", fields...)
	}
	reply.Fail(c, status, code, http.StatusText(status))
}

// userValue 获取日志所需的用户编号。
func userValue(c *gin.Context) uint64 {
	value, _ := c.Get("user_id")
	id, _ := value.(uint64)
	return id
}
