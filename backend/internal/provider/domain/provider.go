// provider.go 定义 AI Provider 的统一请求、事件和流式调用契约。
package provider

import "context"

// Message 表示发送给模型的上下文消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request 保存兼容 OpenAI Chat Completions 的最小请求。
type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// Event 表示一次生成中的增量或终止事件。
type Event struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

// Stream 是 Provider 返回的可取消事件流。
type Stream interface {
	Next(ctx context.Context) (Event, error)
	Close() error
}

// Provider 定义与具体模型供应商无关的生成能力。
type Provider interface {
	Stream(ctx context.Context, req Request) (Stream, error)
}
