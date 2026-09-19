// query.go 提供有界人设列表及本人版本读取。
package personahttp

import (
	"ai-chat/backend/internal/persona/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"strconv"
)

func (h *Handler) list(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 1000000 { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, domain.ErrInvalid)
		return
	}
	out, err := h.service.List(c.Request.Context(), c.GetUint64("user_id"), page)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, gin.H{"items": out, "page": page, "size": 20})
}

func (h *Handler) find(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.Find(c.Request.Context(), c.GetUint64("user_id"), id)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	reply.OK(c, out)
}
