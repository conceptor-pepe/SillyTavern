// slow_test.go 用真实 TCP 慢读客户端验证 SSE 写入不会无限阻塞。
package generationhttp

import (
	"errors"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestSlowFrame 客户端发送请求后完全不读响应，必须由服务端时限结束写入。
func TestSlowFrame(t *testing.T) {
	result := make(chan error, 1)
	engine := gin.New()
	engine.GET("/", func(c *gin.Context) {
		result <- (&Handler{}).send(c, "message_delta", strings.Repeat("x", 8<<20))
	})
	api := httptest.NewServer(engine)
	defer api.Close()
	conn, err := net.DialTimeout("tcp", api.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.(*net.TCPConn).SetReadBuffer(1024); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		var timeout net.Error
		if !errors.As(err, &timeout) || !timeout.Timeout() {
			t.Fatalf("slow reader did not hit write deadline: %v", err)
		}
	case <-time.After(4 * frameLimit):
		t.Fatal("slow reader blocked SSE writer")
	}
}
