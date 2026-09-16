// slow_flow_test.go 使用真实 TLS 和 MySQL 验证慢客户端触发上游关闭及任务失败收尾。
package httpapi

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"ai-chat/backend/internal/model"
)

// TestSlowGeneration 只读取开始事件，随后停止消费响应，不能靠客户端断开触发收尾。
func TestSlowGeneration(t *testing.T) {
	stopped := make(chan struct{})
	f := newFlow(t, floodUpstream(stopped))
	f.login(t)
	f.seedChat(t)
	response := f.open(t, "POST", "/api/v1/chats/"+f.chatID+"/generations",
		`{"model":"test","parent_id":"`+f.msgID+`"}`)
	id := readStart(t, bufio.NewReader(response.Body))
	select {
	case <-stopped:
	case <-time.After(8 * time.Second):
		t.Fatal("slow client did not close model connection")
	}
	waitFailed(t, f, id)
	checkShutdownTask(t, f, id)
}

// floodUpstream 连续输出合法小帧，超过下游接收缓冲后等待请求取消。
func floodUpstream(stopped chan<- struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(stopped)
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		event := `data: {"choices":[{"index":0,"delta":{"content":"` +
			strings.Repeat("x", 16<<10) + `"}}]}` + "\n\n"
		control := http.NewResponseController(w)
		for i := 0; i < 1024; i++ {
			if _, err := io.WriteString(w, event); err != nil {
				return
			}
			if err := control.Flush(); err != nil {
				return
			}
		}
		<-r.Context().Done()
	})
}

// waitFailed 有界等待独立失败事务提交，上游关闭发生在持久化之前。
func waitFailed(t *testing.T, f *flowEnv, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var task model.Generation
		if err := f.db.WithContext(ctx).First(&task, id).Error; err != nil {
			t.Fatal(err)
		}
		if task.Status == "failed" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("slow client left generation running")
		case <-ticker.C:
		}
	}
}
