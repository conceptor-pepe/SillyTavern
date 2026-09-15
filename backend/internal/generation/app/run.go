// run.go 编排 Provider 流式生成和 AI 消息落库。
package app

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
)

// RunArgs 保存一次生成所需的输入。
type RunArgs struct {
	UserID         uint64
	ConversationID uint64
	GenerationID   uint64
	ParentID       *uint64
	Request        provider.Request
}

// Runner 负责执行生成任务并保存完整 AI 消息。
type Runner struct {
	tasks    *Service
	provider provider.Provider
	messages MessageWriter
	mu       sync.Mutex
	stops    map[uint64]context.CancelFunc
}

// MessageWriter 保存生成完成后的 AI 消息。
type MessageWriter interface {
	Create(ctx context.Context, userID uint64, item msgdomain.Message) (msgdomain.Message, error)
}

// NewRunner 创建生成编排器。
func NewRunner(tasks *Service, p provider.Provider, messages MessageWriter) *Runner {
	return &Runner{tasks: tasks, provider: p, messages: messages, stops: make(map[uint64]context.CancelFunc)}
}

// Run 执行流式生成，返回完整的 AI 消息。
func (r *Runner) Run(ctx context.Context, args RunArgs) (msgdomain.Message, error) {
	return r.RunStream(ctx, args, nil)
}

// Cancel 中断指定生成任务对应的 Provider 请求。
func (r *Runner) Cancel(id uint64) {
	r.mu.Lock()
	stop := r.stops[id]
	r.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// RunStream 执行生成并把每个文本增量交给调用方。
func (r *Runner) RunStream(ctx context.Context, args RunArgs, send func(string) error) (item msgdomain.Message, err error) {
	runCtx, stop := context.WithCancel(ctx)
	r.addStop(args.GenerationID, stop)
	defer r.delStop(args.GenerationID)
	defer stop()
	if err := r.tasks.Start(runCtx, args.UserID, args.GenerationID, time.Now().Unix()); err != nil {
		return msgdomain.Message{}, err
	}
	stream, err := r.provider.Stream(runCtx, args.Request)
	if err != nil {
		return r.fail(ctx, args, err)
	}
	defer func() {
		closeErr := stream.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	text, err := readStream(runCtx, stream, send)
	if err != nil {
		return r.fail(ctx, args, err)
	}
	item, err = r.messages.Create(runCtx, args.UserID, msgdomain.Message{
		ConversationID: args.ConversationID, ParentID: args.ParentID,
		Role: "assistant", Content: text, Status: "completed",
	})
	if err != nil {
		return r.fail(ctx, args, err)
	}
	err = r.tasks.Finish(runCtx, args.UserID, args.GenerationID, item.ID, time.Now().Unix())
	return item, err
}

// addStop 注册运行中的任务，供取消接口找到对应请求。
func (r *Runner) addStop(id uint64, stop context.CancelFunc) {
	r.mu.Lock()
	r.stops[id] = stop
	r.mu.Unlock()
}

// delStop 清理已结束任务，避免取消表持续增长。
func (r *Runner) delStop(id uint64) {
	r.mu.Lock()
	delete(r.stops, id)
	r.mu.Unlock()
}

func (r *Runner) fail(ctx context.Context, args RunArgs, cause error) (msgdomain.Message, error) {
	failArgs := FailArgs{
		UserID: args.UserID, ID: args.GenerationID, Code: "generation_failed",
		Message: cause.Error(), Finished: time.Now().Unix(),
	}
	if err := r.tasks.Fail(ctx, failArgs); err != nil {
		return msgdomain.Message{}, errors.Join(cause, err)
	}
	return msgdomain.Message{}, cause
}

func readStream(ctx context.Context, stream provider.Stream, send func(string) error) (string, error) {
	var text string
	for {
		event, err := stream.Next(ctx)
		if err != nil {
			return endStream(text, err)
		}
		text += event.Text
		if err := sendDelta(send, event.Text); err != nil {
			return "", err
		}
		if event.Type == "done" {
			return text, nil
		}
	}
}

// endStream 统一处理流结束，兼容 Provider 以 EOF 表示正常结束。
func endStream(text string, err error) (string, error) {
	if errors.Is(err, io.EOF) {
		return text, nil
	}
	return "", err
}

// sendDelta 发送非空文本增量，避免把回调判断嵌入流循环。
func sendDelta(send func(string) error, text string) error {
	if send == nil || text == "" {
		return nil
	}
	return send(text)
}
