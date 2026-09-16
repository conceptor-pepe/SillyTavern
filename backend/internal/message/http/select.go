// select.go 提供候选回复选择入口，返回独立分支的消息编号供前端继续对话。
package messagehttp

import (
	"strconv"

	"ai-chat/backend/internal/message/app"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// selectVariant 不覆盖原消息，重复选择由事务仓储返回同一个分支。
func (h *Handler) selectVariant(c *gin.Context) {
	uid, messageID, err := readMessage(c)
	if err != nil {
		h.fail(c, err)
		return
	}
	variantID, err := strconv.ParseUint(c.Param("variant"), 10, 64)
	if err != nil || variantID == 0 {
		h.fail(c, app.ErrQuery)
		return
	}
	item, err := h.write.SelectVariant(c.Request.Context(), uid, messageID, variantID)
	if err != nil {
		h.fail(c, err)
		return
	}
	h.logger.Info("message variant selected", zap.Uint64("user_id", uid),
		zap.Uint64("message_id", item.ID), zap.Uint64("source_message_id", messageID),
		zap.Uint64("variant_id", variantID), zap.String("request_id", c.GetString("request_id")))
	reply.OK(c, item)
}
