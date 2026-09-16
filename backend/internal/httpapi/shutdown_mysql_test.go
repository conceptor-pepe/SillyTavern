// shutdown_mysql_test.go 验证停止超时不提前释放活动请求仍需使用的数据库。
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ai-chat/backend/internal/testdb"
)

// TestShutdownTimeout 使用实际 HTTP 连接验证活动请求退出前后数据库的生命周期。
func TestShutdownTimeout(t *testing.T) {
	conn := testdb.Open(t)
	pool, err := conn.DB()
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	unblock := sync.OnceFunc(func() { close(release) })
	drain := newDrain()
	api := httptest.NewServer(drain.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		// 模拟收到取消后仍需使用独立上下文保存终态的业务收尾。
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), time.Second)
		defer cancel()
		result <- pool.PingContext(ctx)
		w.WriteHeader(http.StatusNoContent)
	})))
	defer api.Close()
	defer unblock()
	finished := callShutdown(api)
	awaitRemote(t, entered)
	server := &Server{http: api.Config, sql: conn, drain: drain}
	assertStopTimeout(t, server)
	if err := pool.PingContext(t.Context()); err != nil {
		t.Fatalf("shutdown timeout closed active database: %v", err)
	}
	unblock()
	awaitShutdown(t, result)
	awaitShutdown(t, finished)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.PingContext(t.Context()); err == nil {
		t.Fatal("successful shutdown left database open")
	}
}

// callShutdown 发起有界的真实请求，通过通道把客户端结果交回测试线程。
func callShutdown(api *httptest.Server) <-chan error {
	done := make(chan error, 1)
	client := api.Client()
	client.Timeout = 5 * time.Second
	go func() {
		response, err := client.Get(api.URL)
		if err != nil {
			done <- err
			return
		}
		done <- response.Body.Close()
	}()
	return done
}

// assertStopTimeout 确认第一次关闭确实因活动请求未退出而超时。
func assertStopTimeout(t *testing.T, server *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := server.Stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected shutdown timeout, got %v", err)
	}
}

// awaitShutdown 等待请求或数据库调用完成，避免失败场景永久挂住测试。
func awaitShutdown(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown request did not complete")
	}
}
