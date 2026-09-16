// variants.go 提供消息候选历史的分页查询接口。
package messagehttp

import (
	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
)

// variants 返回候选记录，主消息内容仍通过消息历史接口读取。
func (h *Handler) variants(c *gin.Context) {
	uid, messageID, page, size, err := readArgs(c)
	if err != nil {
		h.fail(c, err)
		return
	}
	items, total, err := h.query.Variants(c.Request.Context(), uid, messageID, page, size)
	if err != nil {
		h.fail(c, err)
		return
	}
	reply.OK(c, struct {
		Items []domain.Variant `json:"items"`
		Total int64            `json:"total,string"`
		Page  int              `json:"page"`
		Size  int              `json:"size"`
	}{items, total, page, size})
}
