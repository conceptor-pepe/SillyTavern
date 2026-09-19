// revision.go 暴露 AI 回复编辑分支入口。
package messagehttp

import (
	"ai-chat/backend/internal/message/app"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// reviseAssistant 把编辑后的 AI 回复保存为新分支。
func (h *Handler) reviseAssistant(c *gin.Context) {
	uid, messageID, err := readMessage(c)
	if err != nil { // audit:allow-no-log 统一失败处理会记录请求上下文。
		h.fail(c, err)
		return
	}
	var in struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&in); err != nil { // audit:allow-no-log 统一失败处理会记录请求上下文。
		h.fail(c, app.ErrQuery)
		return
	}
	item, err := h.write.ReviseAssistant(c.Request.Context(), uid, messageID, in.Content)
	if err != nil { // audit:allow-no-log 统一失败处理会记录请求上下文。
		h.fail(c, err)
		return
	}
	h.logger.Info("assistant reply revised", zap.Uint64("user_id", uid),
		zap.Uint64("source_message_id", messageID), zap.Uint64("message_id", item.ID))
	reply.OK(c, item)
}
