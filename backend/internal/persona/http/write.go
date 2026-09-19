// write.go 人设输入白名单保护归属和修订。
package personahttp

import (
	"ai-chat/backend/internal/persona/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *Handler) save(c *gin.Context) {
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
		Revision    uint64 `json:"expected_revision,string"`
	}
	if err := bind(c, &in); err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	item := domain.Persona{UserID: c.GetUint64("user_id"), Revision: in.Revision, Name: in.Name, Description: in.Description, Avatar: in.Avatar}
	if c.Param("id") != "" {
		id, err := readID(c)
		if err != nil { // audit:allow-no-log 高频输入由错误码说明。
			fail(c, err)
			return
		}
		item.ID = id
	}
	out, err := h.service.Save(c.Request.Context(), item)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	h.logger.Info("persona write completed", zap.Uint64("user_id", c.GetUint64("user_id")))
	reply.OK(c, out)
}

func (h *Handler) remove(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.GetUint64("user_id"), id); err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	h.logger.Info("persona delete completed", zap.Uint64("user_id", c.GetUint64("user_id")))
	reply.OK(c, gin.H{"deleted": true})
}
