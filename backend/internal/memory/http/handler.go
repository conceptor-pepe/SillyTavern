// handler.go 提供记忆与世界书维护接口，归属只来自登录态和路径。
package memoryhttp

import (
	"ai-chat/backend/internal/memory/app"
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/reply"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

// Handler 依赖记忆应用服务，HTTP 层不接触模型或数据库。
type Handler struct{ service *app.Service }

// New 创建记忆接口。
func New(service *app.Service) *Handler { return &Handler{service: service} }

// Routes 世界书与事实共用显式 kind 字段，不增加重复接口。
func (h *Handler) Routes(e *gin.Engine, auth gin.HandlerFunc) {
	e.GET("/api/v1/chats/:id/memory-candidates", auth, h.listCandidates)
	e.POST("/api/v1/chats/:id/memory-candidates", auth, h.extractCandidates)
	e.POST("/api/v1/memory-candidates/:id/accept", auth, h.acceptCandidate)
	e.DELETE("/api/v1/memory-candidates/:id", auth, h.rejectCandidate)
	e.GET("/api/v1/characters/:id/memories", auth, h.list)
	e.POST("/api/v1/characters/:id/memories", auth, h.save)
	e.PUT("/api/v1/characters/:id/memories/:memory", auth, h.save)
	e.DELETE("/api/v1/characters/:id/memories/:memory", auth, h.remove)
	e.GET("/api/v1/chats/:id/memories", auth, h.list)
	e.POST("/api/v1/chats/:id/memories", auth, h.save)
	e.PUT("/api/v1/chats/:id/memories/:memory", auth, h.save)
	e.DELETE("/api/v1/chats/:id/memories/:memory", auth, h.remove)
	e.GET("/api/v1/relationships/:id/memories", auth, h.list)
	e.POST("/api/v1/relationships/:id/memories", auth, h.save)
	e.PUT("/api/v1/relationships/:id/memories/:memory", auth, h.save)
	e.DELETE("/api/v1/relationships/:id/memories/:memory", auth, h.remove)
}

// scope 拒绝无效的编号，客户端不能提交其他用户 ID。
func scope(c *gin.Context) (domain.Entry, error) {
	charID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	item := domain.Entry{UserID: c.GetUint64("user_id"), CharacterID: charID}
	if strings.HasPrefix(c.FullPath(), "/api/v1/chats/") {
		item.ChatID = charID
		item.CharacterID = 0
	}
	if err != nil || charID == 0 {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		return item, domain.ErrInvalid
	}
	if c.Param("memory") != "" {
		item.ID, err = strconv.ParseUint(c.Param("memory"), 10, 64)
		if item.ID == 0 {
			err = domain.ErrInvalid
		}
	}
	return item, err
}

// list 二百条配额使列表可一次完整展示，不隐藏未读取的记忆。
func (h *Handler) list(c *gin.Context) {
	item, err := scope(c)
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		fail(c, err)
		return
	}
	items, err := h.entries(c, item)
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		fail(c, err)
		return
	}
	reply.OK(c, gin.H{"items": items, "limit": 200})
}

// save 明确的字段白名单防止用户伪造数据库归属与向量。
func (h *Handler) save(c *gin.Context) {
	item, err := scope(c)
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		fail(c, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 12000)
	var in struct {
		Kind     string   `json:"kind"`
		Content  string   `json:"content"`
		Keywords []string `json:"keywords"`
		Enabled  bool     `json:"enabled"`
		Pinned   bool     `json:"pinned"`
	}
	in.Enabled = true
	if err := c.ShouldBindJSON(&in); err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		fail(c, domain.ErrInvalid)
		return
	}
	item.Kind, item.Content, item.Keywords, item.Enabled, item.Pinned = in.Kind, in.Content, in.Keywords, in.Enabled, in.Pinned
	var out domain.Entry
	if relationshipPath(c) {
		out, err = h.service.SaveRelationship(c.Request.Context(), item)
	} else {
		out, err = h.service.Save(c.Request.Context(), item)
	}
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		fail(c, err)
		return
	}
	reply.OK(c, out)
}

// remove 服务端立即遗忘指定条目，不能按其他角色的编号删除。
func (h *Handler) remove(c *gin.Context) {
	item, err := scope(c)
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		fail(c, err)
		return
	}
	if err := h.deleteEntry(c, item); err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		fail(c, err)
		return
	}
	reply.OK(c, gin.H{"deleted": true})
}

// fail 返回固定错误，不将原始记忆或供应商错误暴露给页面。
func fail(c *gin.Context, err error) {
	if errors.Is(err, domain.ErrInvalid) {
		reply.Fail(c, 400, "INVALID_MEMORY", "内容最多4000字节，世界书需设置触发词或始终记住；每个角色最多200条，常驻记忆合计最多6000字节")
		return
	}
	if errors.Is(err, domain.ErrNotFound) {
		reply.Fail(c, 404, "NOT_FOUND", "角色或记忆不存在")
		return
	}
	reply.Fail(c, 500, "MEMORY_ERROR", "记忆暂不可用，请稍后重试")
}

func (h *Handler) entries(c *gin.Context, item domain.Entry) ([]domain.Entry, error) {
	if item.ChatID != 0 {
		return h.service.ListChat(c.Request.Context(), item.UserID, item.ChatID)
	}
	if relationshipPath(c) {
		return h.service.ListRelationship(c.Request.Context(), item.UserID, item.CharacterID)
	}
	return h.service.List(c.Request.Context(), item.UserID, item.CharacterID)
}

func (h *Handler) deleteEntry(c *gin.Context, item domain.Entry) error {
	if item.ChatID != 0 {
		return h.service.DeleteChat(c.Request.Context(), item)
	}
	if relationshipPath(c) {
		return h.service.DeleteRelationship(c.Request.Context(), item)
	}
	return h.service.Delete(c.Request.Context(), item.UserID, item.CharacterID, item.ID)
}

func relationshipPath(c *gin.Context) bool {
	return strings.HasPrefix(c.FullPath(), "/api/v1/relationships/")
}
