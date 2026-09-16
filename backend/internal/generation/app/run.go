// run.go 编排 Provider 流式生成和 AI 消息落库。
package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"ai-chat/backend/internal/generation/domain"
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
	limit    time.Duration
}

// MessageWriter 保存生成完成后的 AI 消息。
type MessageWriter interface {
	Create(ctx context.Context, userID uint64, item msgdomain.Message) (msgdomain.Message, error)
}

// DoneWriter 在同一事务中保存 assistant 消息并完成生成任务。
type DoneWriter interface {
	SaveDone(ctx context.Context, userID, generationID uint64, item msgdomain.Message, finished int64) (msgdomain.Message, error)
}

// NewRunner 创建生成编排器。
func NewRunner(tasks *Service, p provider.Provider, messages MessageWriter) *Runner {
	return &Runner{tasks: tasks, provider: p, messages: messages,
		stops: make(map[uint64]context.CancelFunc), limit: domain.RunLimit}
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
	return r.RunEvents(ctx, args, func(event provider.Event) error {
		if event.Index != 0 {
			return nil
		}
		return sendDelta(send, event.Text)
	})
}

// RunEvents 按候选索引推送增量，并在完整读取后统一落库。
func (r *Runner) RunEvents(ctx context.Context, args RunArgs, send func(provider.Event) error) (msgdomain.Message, error) {
	timedCtx, stop := context.WithTimeout(ctx, r.limit)
	defer stop()
	runCtx, cancel := context.WithCancelCause(timedCtx)
	defer cancel(nil)
	r.addStop(args.GenerationID, func() { cancel(context.Canceled) })
	defer r.delStop(args.GenerationID)
	if err := r.tasks.Start(runCtx, args.UserID, args.GenerationID, time.Now().Unix()); err != nil {
		return r.fail(runCtx, args, err)
	}
	stopWatch := r.watchTask(runCtx, args, cancel)
	defer stopWatch()
	msg, err := r.readReply(runCtx, args, send)
	// 上游已关闭，先退出监听再完成事务，避免将自身刚提交的 completed 误当外部取消。
	stopWatch()
	if err != nil {
		return r.fail(runCtx, args, err)
	}
	item, err := r.saveResult(runCtx, args, msg)
	if err != nil {
		return r.fail(runCtx, args, err)
	}
	return item, nil
}

// readReply 读取并关闭上游后组装候选；完整结果由调用方统一提交。
func (r *Runner) readReply(ctx context.Context, args RunArgs, send func(provider.Event) error) (msgdomain.Message, error) {
	stream, err := r.provider.Stream(ctx, args.Request)
	if err != nil {
		return msgdomain.Message{}, err
	}
	variants, readErr := readCandidates(ctx, stream, args.Request.N, send)
	err = errors.Join(readErr, stream.Close())
	if err != nil {
		return msgdomain.Message{}, err
	}
	msg := msgdomain.Message{
		ConversationID: args.ConversationID, ParentID: args.ParentID,
		Role: "assistant", Content: variants[0].Content, Status: "completed",
	}
	if len(variants) > 1 {
		msg.Variants = variants
	}
	return msg, nil
}

// saveResult 要求多候选使用事务写入器，禁止先完成任务再追加候选。
func (r *Runner) saveResult(ctx context.Context, args RunArgs, msg msgdomain.Message) (msgdomain.Message, error) {
	if writer, ok := r.messages.(DoneWriter); ok {
		return writer.SaveDone(ctx, args.UserID, args.GenerationID, msg, time.Now().Unix())
	}
	if len(msg.Variants) > 1 {
		return msgdomain.Message{}, errors.New("candidate transaction writer required")
	}
	item, err := r.messages.Create(ctx, args.UserID, msg)
	if err != nil {
		return msgdomain.Message{}, err
	}
	err = r.tasks.Finish(ctx, args.UserID, args.GenerationID, item.ID, time.Now().Unix())
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

// fail 使用独立短超时保存失败状态，客户端断开不应阻止任务收尾。
func (r *Runner) fail(ctx context.Context, args RunArgs, cause error) (msgdomain.Message, error) {
	// 关闭响应体可能仅返回网络读取错误，必须保留实际运行上下文的超时原因。
	cause = errors.Join(cause, ctx.Err(), context.Cause(ctx))
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	failArgs := FailArgs{
		UserID: args.UserID, ID: args.GenerationID, Code: "generation_failed",
		Message: "generation failed", Finished: time.Now().Unix(),
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		failArgs.Code, failArgs.Message = "generation_timeout", "generation timed out"
	}
	if err := r.tasks.Fail(ctx, failArgs); err != nil {
		return msgdomain.Message{}, errors.Join(cause, err)
	}
	return msgdomain.Message{}, cause
}

// sendDelta 发送非空文本增量，避免把回调判断嵌入流循环。
func sendDelta(send func(string) error, text string) error {
	if send == nil || text == "" {
		return nil
	}
	return send(text)
}
