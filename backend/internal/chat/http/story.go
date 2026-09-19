// story.go 将故事开聊入口适配到会话应用层。
package chathttp

import (
	"ai-chat/backend/internal/chat/app"
	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"strconv"
)

// UseStory 装配故事能力，旧测试或调用方可继续只启用角色聊天。
// @param service 故事服务
func (h *Handler) UseStory(service *app.StoryService) { h.story = service }

func (h *Handler) startStory(c *gin.Context, in domain.StoryStart) {
	if h.story == nil {
		reply.Fail(c, 503, "UNAVAILABLE", "故事暂不可用")
		return
	}
	out, err := h.story.Start(c.Request.Context(), in)
	if err != nil { // audit:allow-no-log 应用层及统一失败处理记录。
		h.fail(c, err)
		return
	}
	reply.OK(c, out)
}

func (h *Handler) bootstrap(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 { // audit:allow-no-log 高频输入由统一错误处理记录。
		h.fail(c, app.ErrQuery)
		return
	}
	if h.story == nil {
		reply.Fail(c, 503, "UNAVAILABLE", "故事暂不可用")
		return
	}
	out, err := h.story.State(c.Request.Context(), userID(c), id)
	if err != nil { // audit:allow-no-log 应用层及统一失败处理记录。
		h.fail(c, err)
		return
	}
	reply.OK(c, out)
}
