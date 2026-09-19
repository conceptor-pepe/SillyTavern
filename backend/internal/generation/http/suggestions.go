// suggestions.go 提供不写入剧情历史的玩家回复建议。
package generationhttp

import (
	"net/http"

	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// suggestions 基于指定 AI 回复返回三个玩家可选回复。
func (h *Handler) suggestions(c *gin.Context) {
	uid, chatID, err := readIDs(c)
	if err != nil { // audit:allow-no-log 固定输入错误不记录堆栈。
		reply.Fail(c, http.StatusBadRequest, "INVALID_QUERY", "invalid query")
		return
	}
	var in struct {
		Model    string  `json:"model"`
		ParentID *uint64 `json:"parent_id,string" binding:"required"`
	}
	in.Model = h.defaultModel
	if err := c.ShouldBindJSON(&in); err != nil || in.Model == "" || in.ParentID == nil {
		h.logger.Warn("reply suggestion body rejected", zap.Uint64("user_id", uid), zap.Uint64("chat_id", chatID))
		reply.Fail(c, http.StatusBadRequest, "INVALID_BODY", "invalid body")
		return
	}
	if h.suggester == nil {
		reply.Fail(c, http.StatusServiceUnavailable, "UNAVAILABLE", "suggestions unavailable")
		return
	}
	prompt, err := h.prompt(c, promptArgs{
		uid: uid, chatID: chatID, parentID: in.ParentID, leafRole: "assistant",
	})
	if err != nil { // audit:allow-no-log 统一失败处理记录上下文。
		h.fail(c, uid, 0, err)
		return
	}
	items, err := h.suggester.Suggest(c.Request.Context(), in.Model, prompt, suggestionTokens(h.outputTokens))
	if err != nil {
		h.logger.Error("reply suggestions failed", zap.Uint64("user_id", uid),
			zap.Uint64("chat_id", chatID), zap.Error(err))
		reply.Fail(c, http.StatusBadGateway, "SUGGESTION_FAILED", "suggestion generation failed")
		return
	}
	reply.OK(c, struct {
		AnchorID uint64   `json:"anchor_id,string"`
		Items    []string `json:"items"`
	}{AnchorID: *in.ParentID, Items: items})
}

// suggestionTokens 限制辅助请求成本，配置更小时服从全局上限。
func suggestionTokens(configured int) int {
	if configured <= 0 || configured > 320 {
		return 320
	}
	return configured
}
