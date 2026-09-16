// drain.go 在停机时关闭请求入口并等待业务 Handler 完成独立收尾。
package httpapi

import (
	"context"
	"net/http"
	"sync"
)

// requestDrain 保护请求计数和停止状态，禁止停止后新请求进入业务层。
type requestDrain struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	active  int
	stopped bool
	done    chan struct{}
}

// newDrain 创建服务私有的取消源，不改变请求原有的身份和截止时间。
func newDrain() *requestDrain {
	ctx, cancel := context.WithCancel(context.Background())
	return &requestDrain{ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

// wrap 登记完整 Handler 生命周期，取消信号本身不能代替业务收尾完成。
func (d *requestDrain) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !d.enter() {
			http.Error(w, "server stopping", http.StatusServiceUnavailable)
			return
		}
		defer d.leave()
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		stop := context.AfterFunc(d.ctx, cancel)
		defer stop()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// enter 与 stop 使用同一把锁，避免停机等待结束后又增加活动请求。
func (d *requestDrain) enter() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return false
	}
	d.active++
	return true
}

// leave 只在最后一个已登记请求退出时通知停止等待者。
func (d *requestDrain) leave() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.active--
	if d.stopped && d.active == 0 {
		close(d.done)
	}
}

// stop 幂等关闭入口并取消活动请求，真正结束仍需等待 done。
func (d *requestDrain) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopped {
		return
	}
	d.stopped = true
	d.cancel()
	if d.active == 0 {
		close(d.done)
	}
}

// wait 在调用方时限内等待请求退出，超时不得提前释放业务依赖。
func (d *requestDrain) wait(ctx context.Context) error {
	select {
	case <-d.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
