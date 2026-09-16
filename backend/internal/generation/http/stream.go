// stream.go 负责 SSE 编码、写入错误检测和首帧断连后的任务收尾。
package generationhttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"ai-chat/backend/internal/generation/domain"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// frameLimit 限制单帧传输阻塞，为停机后的数据库收尾保留时间。
const frameLimit = 2 * time.Second

// open 设置事件流响应头，后续写入负责实际发送。
func (h *Handler) open(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
}

// send 先编码再检查完整帧的写入结果，避免框架隐藏连接错误。
func (h *Handler) send(c *gin.Context, name string, data any) error {
	if err := c.Request.Context().Err(); err != nil {
		return err
	}
	var frame bytes.Buffer
	if err := sse.Encode(&frame, sse.Event{Event: name, Data: data}); err != nil {
		return err
	}
	return writeFrame(c, frame.Bytes())
}

// writeFrame 保留 Gin 写入统计，但绕过其不返回错误的 Flush，逐帧设置和清除写时限。
func writeFrame(c *gin.Context, frame []byte) (err error) {
	writer := http.ResponseWriter(c.Writer)
	if wrapped, ok := writer.(interface{ Unwrap() http.ResponseWriter }); ok {
		writer = wrapped.Unwrap()
	}
	control := http.NewResponseController(writer)
	if err := control.SetWriteDeadline(time.Now().Add(frameLimit)); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, control.SetWriteDeadline(time.Time{})) }()
	n, err := c.Writer.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	if err := control.Flush(); err != nil {
		return err
	}
	return c.Request.Context().Err()
}

// cancelStart 在 Runner 尚未启动时收尾，不能复用已经取消的请求上下文。
func (h *Handler) cancelStart(c *gin.Context, args runArgs) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
	defer cancel()
	fields := []zap.Field{zap.Uint64("user_id", args.uid), zap.Uint64("chat_id", args.chatID),
		zap.Uint64("generation_id", args.task.ID), zap.String("request_id", c.GetString("request_id"))}
	err := h.tasks.Cancel(ctx, args.uid, args.task.ID)
	if errors.Is(err, domain.ErrNotFound) {
		h.logger.Info("generation already ended", fields...)
		return
	}
	if err != nil {
		h.logger.Error("generation disconnect cleanup failed", append(fields, zap.Error(err))...)
		return
	}
	h.logger.Info("generation cancelled before start", fields...)
}
