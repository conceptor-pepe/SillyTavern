// handler.go 提供按本人会话访问关系档案的接口。
package relationshiphttp

import (
	"ai-chat/backend/internal/relationship/app"
	"ai-chat/backend/internal/relationship/domain"
	"ai-chat/backend/internal/reply"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 将关系归属固定为登录用户和本人会话。
type Handler struct{ service *app.Service }

// New 创建关系接口。
// @param service 关系服务
// @return 关系接口
func New(service *app.Service) *Handler { return &Handler{service: service} }

// Routes 注册会话关系读取和更新路由。
// @param e 路由引擎
// @param auth 鉴权中间件
func (h *Handler) Routes(e *gin.Engine, auth gin.HandlerFunc) {
	e.GET("/api/v1/chats/:id/relationship", auth, h.find)
	e.PUT("/api/v1/chats/:id/relationship", auth, h.update)
}

func (h *Handler) find(c *gin.Context) {
	uid, chatID, err := ids(c)
	if err != nil { // audit:allow-no-log 高频输入由固定错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.ByChat(c.Request.Context(), uid, chatID)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, out)
}

func (h *Handler) update(c *gin.Context) {
	uid, chatID, err := ids(c)
	if err != nil { // audit:allow-no-log 高频输入由固定错误码说明。
		fail(c, err)
		return
	}
	var in struct {
		Stage      string   `json:"stage"`
		Narrative  string   `json:"narrative"`
		Milestones []string `json:"milestones"`
		Revision   uint64   `json:"expected_revision,string"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 12000)
	if c.ShouldBindJSON(&in) != nil { // audit:allow-no-log 高频输入由固定错误码说明。
		fail(c, domain.ErrInvalid)
		return
	}
	out, err := h.service.UpdateByChat(c.Request.Context(), domain.Update{UserID: uid, ChatID: chatID,
		Stage: in.Stage, Narrative: in.Narrative, Milestones: in.Milestones, Revision: in.Revision})
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, out)
}

func ids(c *gin.Context) (uint64, uint64, error) {
	chatID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	uid := c.GetUint64("user_id")
	if err != nil || uid == 0 || chatID == 0 {
		// audit:allow-no-log 高频路径参数校验由固定错误码说明。
		return 0, 0, domain.ErrInvalid
	}
	return uid, chatID, nil
}

func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		reply.Fail(c, 400, "INVALID_RELATIONSHIP", "关系资料无效")
	case errors.Is(err, domain.ErrNotFound):
		reply.Fail(c, 404, "NOT_FOUND", "当前会话没有可共享的角色关系")
	case errors.Is(err, domain.ErrConflict):
		reply.Fail(c, 409, "REVISION_CONFLICT", "关系档案已经更新，请刷新后重试")
	default:
		reply.Fail(c, 500, "RELATIONSHIP_ERROR", "关系档案暂不可用")
	}
}
