// models_test.go 验证摘要流完整性，断流不能留下看似成功的半段摘要。
package infra

import (
	provider "ai-chat/backend/internal/provider/domain"
	"context"
	"io"
	"testing"
)

type summaryStream struct {
	sent     bool
	complete bool
	closed   bool
}

// Next 模拟正常结束和意外断流。
func (s *summaryStream) Next(context.Context) (provider.Event, error) {
	if !s.sent {
		s.sent = true
		return provider.Event{Type: "delta", Text: "用户喜欢书店。"}, nil
	}
	if s.complete {
		return provider.Event{Type: "done"}, nil
	}
	return provider.Event{Type: "done"}, io.EOF
}

// Close 记录资源释放结果。
func (s *summaryStream) Close() error { s.closed = true; return nil }

// TestSummaryCompletion 正常结束保存摘要，意外 EOF 拒绝并关闭连接。
func TestSummaryCompletion(t *testing.T) {
	for _, complete := range []bool{true, false} {
		stream := &summaryStream{complete: complete}
		text, err := collectSummary(t.Context(), stream)
		if complete && (err != nil || text != "用户喜欢书店。") {
			t.Fatal("valid summary rejected")
		}
		if !complete && err == nil {
			t.Fatal("partial summary accepted")
		}
		if !stream.closed {
			t.Fatal("summary stream leaked")
		}
	}
}
