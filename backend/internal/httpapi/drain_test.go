// drain_test.go 验证取消请求与业务完成分离，停机后不会再进入业务代码。
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestDrainWait 验证请求收到取消后仍须等待业务收尾，超时可重试。
func TestDrainWait(t *testing.T) {
	drain := newDrain()
	entered, cancelled, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	defer drain.stop()
	handler := drain.wrap(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(cancelled)
		<-release
	}))
	go func() {
		defer close(done)
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	awaitRemote(t, entered)
	drain.stop()
	awaitRemote(t, cancelled)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if err := drain.wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("returned before business cleanup: %v", err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("accepted request during shutdown: %d", rec.Code)
	}
	unblock()
	awaitRemote(t, done)
	if err := drain.wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	drain.stop()
}

// TestDrainEmpty 验证没有活动请求时停止立即完成且可重复调用。
func TestDrainEmpty(t *testing.T) {
	drain := newDrain()
	drain.stop()
	drain.stop()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := drain.wait(ctx); err != nil {
		t.Fatal(err)
	}
	if drain.enter() {
		t.Fatal("stopped drain admitted request")
	}
}
