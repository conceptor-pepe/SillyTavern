// handler.go 提供 AI 生成的 SSE 接口。
package generationhttp

import (
	"errors"
	"net/http"
	"strconv"

	chardomain "ai-chat/backend/internal/character/domain"
	chatdomain "ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/generation/app"
	"ai-chat/backend/internal/generation/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 保存生成用例和日志依赖。
type Handler struct {
	tasks  *app.Service
	runner *app.Runner
	chats  chatdomain.Repo
	chars  chardomain.Repo
	msgs   msgdomain.Repo
	logger *zap.Logger
}

// Deps 保存生成 Handler 的跨域查询依赖。
type Deps struct {
	Chats chatdomain.Repo
	Chars chardomain.Repo
	Msgs  msgdomain.Repo
}

// runArgs 保存 SSE 生成执行所需参数，避免 Handler 方法参数过多。
type runArgs struct {
	uid      uint64
	chatID   uint64
	task     domain.Generation
	parentID *uint64
	model    string
	count    int
	prompt   []provider.Message
}

// New 创建生成 SSE Handler。
func New(tasks *app.Service, runner *app.Runner, deps Deps, logger *zap.Logger) *Handler {
	return &Handler{tasks: tasks, runner: runner, chats: deps.Chats, chars: deps.Chars, msgs: deps.Msgs, logger: logger}
}

// Routes 注册生成接口。
func (h *Handler) Routes(engine *gin.Engine, auth gin.HandlerFunc) {
	engine.POST("/api/v1/chats/:id/generations", auth, h.stream)
	engine.POST("/api/v1/messages/:id/regenerate", auth, h.regenerate)
	engine.GET("/api/v1/generations/:id", auth, h.find)
	engine.DELETE("/api/v1/generations/:id", auth, h.cancel)
}

// regenerate 根据旧 AI 消息的父消息创建新的生成任务，不覆盖原消息。
func (h *Handler) regenerate(c *gin.Context) {
	uid, messageID, err := readMessageID(c)
	if err != nil {
		h.fail(c, uid, 0, err)
		return
	}
	var in struct {
		Model string `json:"model"`
		N     int    `json:"n" binding:"gte=0,lte=4"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "model is required"})
		return
	}
	finder, ok := h.msgs.(msgdomain.Finder)
	if !ok {
		h.fail(c, uid, messageID, errors.New("message finder unavailable"))
		return
	}
	old, err := finder.Find(c.Request.Context(), uid, messageID)
	if err != nil || old.Role != "assistant" {
		h.fail(c, uid, messageID, errors.New("assistant message not found"))
		return
	}
	if old.ParentID == nil {
		h.fail(c, uid, messageID, app.ErrBranch)
		return
	}
	prompt, err := h.prompt(c, promptArgs{uid: uid, chatID: old.ConversationID, parentID: old.ParentID})
	if err != nil {
		h.fail(c, uid, messageID, err)
		return
	}
	task, err := h.create(c, uid, old.ConversationID, in.Model)
	if err != nil {
		h.fail(c, uid, messageID, err)
		return
	}
	h.run(c, runArgs{uid: uid, chatID: old.ConversationID, task: task, parentID: old.ParentID, model: in.Model, count: in.N, prompt: prompt})
}

// find 返回当前用户可见的生成任务状态。
func (h *Handler) find(c *gin.Context) {
	uid, id, err := readIDs(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_QUERY", "message": "invalid query"})
		return
	}
	item, err := h.tasks.Find(c.Request.Context(), uid, id)
	if errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "generation not found"})
		return
	}
	if err != nil {
		h.fail(c, uid, id, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "OK", "message": "ok", "data": item})
}

// stream 创建任务并推送模型增量事件。
func (h *Handler) stream(c *gin.Context) {
	uid, chatID, err := readIDs(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_QUERY", "message": "invalid query"})
		return
	}
	var in struct {
		Model    string             `json:"model"`
		N        int                `json:"n" binding:"gte=0,lte=4"`
		Content  string             `json:"content"`
		Messages []provider.Message `json:"messages"`
		ParentID *uint64            `json:"parent_id,string"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BODY", "message": "invalid body"})
		return
	}
	owned, err := h.chats.Owns(c.Request.Context(), uid, chatID)
	if err != nil || !owned {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "chat not found"})
		return
	}
	content := lastUser(in.Content, in.Messages)
	prompt, err := h.prompt(c, promptArgs{uid: uid, chatID: chatID, parentID: in.ParentID, content: content})
	if err != nil {
		h.fail(c, uid, 0, err)
		return
	}
	task, err := h.create(c, uid, chatID, in.Model)
	if err != nil {
		h.fail(c, uid, task.ID, err)
		return
	}
	h.run(c, runArgs{uid: uid, chatID: chatID, task: task, parentID: in.ParentID, model: in.Model, count: in.N, prompt: prompt})
}

// run 推送生成事件并执行 Provider 流式任务。
func (h *Handler) run(c *gin.Context, args runArgs) {
	h.open(c)
	if err := h.send(c, "message_start", gin.H{"generation_id": args.task.ID}); err != nil {
		h.logSend(args.uid, args.task.ID, err)
		return
	}
	item, err := h.runner.RunEvents(c.Request.Context(), app.RunArgs{
		UserID: args.uid, ConversationID: args.chatID, GenerationID: args.task.ID,
		ParentID: args.parentID, Request: provider.Request{Model: args.model, Messages: args.prompt, Stream: true, N: args.count},
	}, func(event provider.Event) error {
		return h.send(c, "message_delta", gin.H{"text": event.Text, "index": event.Index})
	})
	if err != nil {
		h.logger.Error("generation run failed", zap.Uint64("user_id", args.uid),
			zap.Uint64("chat_id", args.chatID), zap.Uint64("generation_id", args.task.ID),
			zap.Error(err))
		h.sendErr(c, args.uid, args.task.ID)
		return
	}
	if err := h.send(c, "message_end", gin.H{"message_id": item.ID, "generation_id": args.task.ID, "variants": item.Variants}); err != nil {
		h.logSend(args.uid, args.task.ID, err)
	}
}

// sendErr 向客户端发送统一的生成失败事件。
func (h *Handler) sendErr(c *gin.Context, uid, taskID uint64) {
	if err := h.send(c, "generation_error", gin.H{"message": "generation failed"}); err != nil {
		h.logSend(uid, taskID, err)
	}
}

// create 创建待执行的生成任务。
func (h *Handler) create(c *gin.Context, uid, chatID uint64, model string) (domain.Generation, error) {
	return h.tasks.Create(c.Request.Context(), domain.Generation{
		UserID: uid, ConversationID: chatID, Provider: "openai", Model: model,
	})
}

// lastUser 兼容旧客户端，只提取最后一条用户输入。
func lastUser(content string, items []provider.Message) string {
	if content != "" {
		return content
	}
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Role == "user" {
			return items[i].Content
		}
	}
	return ""
}

func (h *Handler) open(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
}

func (h *Handler) send(c *gin.Context, name string, data any) error {
	c.SSEvent(name, data)
	c.Writer.Flush()
	return c.Request.Context().Err()
}

// logSend 记录 SSE 客户端断开等发送错误。
func (h *Handler) logSend(uid, taskID uint64, err error) {
	h.logger.Warn("generation stream send failed", zap.Uint64("user_id", uid),
		zap.Uint64("generation_id", taskID), zap.Error(err))
}

// fail 返回稳定错误并记录上下文，不向客户端泄漏内部存储错误。
func (h *Handler) fail(c *gin.Context, uid, taskID uint64, err error) {
	if errors.Is(err, app.ErrBranch) || errors.Is(err, msgdomain.ErrNotFound) {
		h.logger.Warn("generation branch rejected", zap.Uint64("user_id", uid), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_BRANCH", "message": "invalid message branch"})
		return
	}
	h.logger.Error("generation create failed", zap.Uint64("user_id", uid),
		zap.Uint64("generation_id", taskID), zap.Error(err))
	c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "internal error"})
}

func readIDs(c *gin.Context) (uint64, uint64, error) {
	value, ok := c.Get("user_id")
	uid, valid := value.(uint64)
	chatID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if !ok || !valid || uid == 0 || err != nil || chatID == 0 {
		return 0, 0, errors.New("invalid ids")
	}
	return uid, chatID, nil
}

// readMessageID 读取鉴权用户和消息编号。
func readMessageID(c *gin.Context) (uint64, uint64, error) {
	value, ok := c.Get("user_id")
	uid, valid := value.(uint64)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if !ok || !valid || uid == 0 || err != nil || id == 0 {
		return 0, 0, errors.New("invalid message id")
	}
	return uid, id, nil
}
