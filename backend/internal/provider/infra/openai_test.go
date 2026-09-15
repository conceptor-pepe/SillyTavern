// openai_test.go 验证 OpenAI 兼容 SSE 的增量文本和结束事件解析。
package infra

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	provider "ai-chat/backend/internal/provider/domain"
)

// TestStream 验证 Provider 能读取增量和结束标记。
func TestStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()
	client := OpenAI{URL: server.URL}
	stream, err := client.Stream(context.Background(), provider.Request{Model: "demo", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	event, err := stream.Next(context.Background())
	if err != nil || event.Type != "delta" || event.Text != "hi" {
		t.Fatalf("event=%+v err=%v", event, err)
	}
	event, err = stream.Next(context.Background())
	if err != nil || event.Type != "done" {
		t.Fatalf("done=%+v err=%v", event, err)
	}
}

// TestStatus 验证非成功状态被转换为稳定错误。
func TestStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "secret provider detail", http.StatusBadGateway)
	}))
	defer server.Close()
	_, err := (&OpenAI{URL: server.URL}).Stream(context.Background(), provider.Request{})
	if err == nil || !strings.Contains(err.Error(), "provider request failed") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("provider detail leaked: %v", err)
	}
}

// TestMalformed 验证畸形 SSE JSON 返回解析错误。
func TestMalformed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {bad}\n\n"))
	}))
	defer server.Close()
	stream, err := (&OpenAI{URL: server.URL}).Stream(context.Background(), provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Next(context.Background()); err == nil {
		t.Fatal("expected malformed json error")
	}
}

// TestCancel 验证取消上下文可以打断响应体读取。
func TestCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := (&OpenAI{URL: server.URL}).Stream(ctx, provider.Request{})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	done := make(chan error, 1)
	go func() {
		_, err := stream.Next(ctx)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not stop")
	}
}
