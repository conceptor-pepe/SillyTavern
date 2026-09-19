// prompt.go 将 HTTP 请求转换为角色上下文和受归属保护的消息分支。
package generationhttp

import (
	"errors"

	"ai-chat/backend/internal/generation/app"
	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"github.com/gin-gonic/gin"
)

// promptArgs 区分已保存的用户父消息与旧客户端直接传入的内容。
type promptArgs struct {
	uid      uint64
	chatID   uint64
	parentID *uint64
	content  string
	leafRole string
}

// prompt 查询角色资料并组装当前分支，父节点已包含的用户文本不重复追加。
func (h *Handler) prompt(c *gin.Context, args promptArgs) ([]provider.Message, error) {
	chat, err := h.chats.Find(c.Request.Context(), args.uid, args.chatID)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return nil, err
	}
	if chat.Mode == "story" {
		if h.story == nil {
			return nil, errors.New("story context unavailable")
		}
		history, err := h.history(c, args)
		if err != nil {
			// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
			return nil, err
		}
		return h.story.Build(c.Request.Context(), args.uid, args.chatID, history)
	}
	char, err := h.chars.Find(c.Request.Context(), args.uid, chat.CharacterID)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return nil, err
	}
	history, err := h.history(c, args)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return nil, err
	}
	if h.memory != nil {
		return h.memory.Build(c.Request.Context(), char, history)
	}
	return app.BuildPrompt(app.PromptArgs{Character: char, History: history}), nil
}

// history 对有父节点请求强制使用祖先链，无父节点仍保留旧客户端兼容路径。
func (h *Handler) history(c *gin.Context, args promptArgs) ([]msgdomain.Message, error) {
	if args.parentID != nil {
		reader, ok := h.msgs.(msgdomain.BranchReader)
		if !ok {
			return nil, errors.New("branch reader unavailable")
		}
		return app.LoadBranch(c.Request.Context(), reader, app.BranchArgs{
			UserID: args.uid, ChatID: args.chatID, LeafID: *args.parentID,
			Content: args.content, LeafRole: args.leafRole, Logger: h.logger,
		})
	}
	if h.memory != nil {
		return nil, app.ErrBranch
	}
	history, _, err := h.msgs.List(c.Request.Context(), args.uid, args.chatID, 1, 100)
	if err != nil {
		// audit:allow-no-log HTTP 错误由统一失败处理记录；仓储错误交调用方记录。
		return nil, err
	}
	if args.content != "" {
		history = append(history, msgdomain.Message{Role: "user", Content: args.content})
	}
	return history, nil
}
