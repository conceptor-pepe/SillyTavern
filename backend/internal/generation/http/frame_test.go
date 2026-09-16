// frame_test.go 验证逐帧写时限、刷新错误和不支持写时限时的明确失败。
package generationhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ai-chat/backend/internal/generation/domain"
	"github.com/gin-gonic/gin"
)

// frameRecorder 为内存响应补充可观察的写时限与刷新错误，不能代替实际网络验收。
type frameRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
	setErr    error
	flushErr  error
}

// newFrameRecorder 创建支持时限记录的响应桩，现有流式测试显式使用它。
func newFrameRecorder() *frameRecorder {
	return &frameRecorder{ResponseRecorder: httptest.NewRecorder()}
}

// SetWriteDeadline 记录设置和清除时限，允许测试模拟传输能力失败。
func (r *frameRecorder) SetWriteDeadline(value time.Time) error {
	r.deadlines = append(r.deadlines, value)
	return r.setErr
}

// FlushError 暴露刷新错误，验证调用方没有落入 Gin 的无错误 Flush。
func (r *frameRecorder) FlushError() error {
	r.Flush()
	return r.flushErr
}

// TestFrameDeadline 验证成功和刷新失败都会清除当前帧时限，不影响下一次模型等待。
func TestFrameDeadline(t *testing.T) {
	failure := errors.New("flush failed")
	for _, cause := range []error{nil, failure} {
		rec := newFrameRecorder()
		rec.flushErr = cause
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest("GET", "/", nil)
		start := time.Now()
		err := (&Handler{}).send(ctx, "message_delta", "hello")
		if !errors.Is(err, cause) {
			t.Fatalf("flush result=%v want=%v", err, cause)
		}
		if len(rec.deadlines) != 2 || !rec.deadlines[1].IsZero() ||
			rec.deadlines[0].Before(start.Add(frameLimit)) || time.Until(rec.deadlines[0]) > frameLimit {
			t.Fatalf("invalid per-frame deadlines: %v", rec.deadlines)
		}
		if ctx.Writer.Size() != rec.Body.Len() || !rec.Flushed {
			t.Fatal("Gin size or flush was bypassed")
		}
	}
}

// TestFrameUnsupported 验证缺少时限能力不会退化为无界写入，也不会发送任何帧。
func TestFrameUnsupported(t *testing.T) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	err := (&Handler{}).send(ctx, "message_start", "hello")
	if !errors.Is(err, http.ErrNotSupported) || rec.Body.Len() != 0 {
		t.Fatalf("unsupported transport err=%v body=%s", err, rec.Body.String())
	}
}

// TestStartFlushError 验证首帧刷新失败会取消 pending，不调用 Provider。
func TestStartFlushError(t *testing.T) {
	tasks := &httpTasks{}
	engine := makeEngine(tasks, httpProvider{err: errors.New("provider must not run")})
	rec := newFrameRecorder()
	rec.flushErr = errors.New("connection flush failed")
	req := httptest.NewRequest("POST", "/api/v1/chats/3/generations",
		strings.NewReader(`{"model":"demo"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)
	if tasks.item.Status != domain.StatusCancelled {
		t.Fatalf("first flush failure left task in %s", tasks.item.Status)
	}
}

// TestFrameSetError 验证无法建立写时限时立即失败，不写入或刷新响应。
func TestFrameSetError(t *testing.T) {
	rec := newFrameRecorder()
	rec.setErr = errors.New("deadline unavailable")
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	err := (&Handler{}).send(ctx, "message_start", "hello")
	if !errors.Is(err, rec.setErr) {
		t.Fatalf("deadline error lost: %v", err)
	}
	if len(rec.deadlines) != 1 || rec.Body.Len() != 0 || rec.Flushed {
		t.Fatal("failed deadline setup wrote or flushed a frame")
	}
}
