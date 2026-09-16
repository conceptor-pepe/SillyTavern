// choices_test.go 验证同帧多候选读取顺序、取消边界和异常帧处理。
package infra

import (
	"bufio"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestChoiceFrames 验证多候选不会丢失，空候选帧不会导致越界或提前终止。
func TestChoiceFrames(t *testing.T) {
	input := `data: {"choices":[{"index":1,"delta":{"content":"B"}},{"index":0,"delta":{"content":"A"}}]}

data: {"choices":[]}

data: {"choices":[{"index":0,"delta":{"content":"a"}},{"index":1,"delta":{"content":"b"}}]}

data: [DONE]
`
	s := &stream{scan: bufio.NewScanner(strings.NewReader(input))}
	want := []struct {
		index int
		text  string
		kind  string
	}{{1, "B", "delta"}, {0, "A", "delta"}, {0, "", "delta"},
		{0, "a", "delta"}, {1, "b", "delta"}, {0, "", "done"}}
	for _, expected := range want {
		got, err := s.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got.Index != expected.index || got.Text != expected.text || got.Type != expected.kind {
			t.Fatalf("got=%+v want=%+v", got, expected)
		}
	}
}

// TestQueuedCancel 验证取消后不继续发送已缓存候选，也不消耗队列。
func TestQueuedCancel(t *testing.T) {
	input := `data: {"choices":[{"index":0,"delta":{"content":"A"}},{"index":1,"delta":{"content":"B"}}]}` + "\n"
	s := &stream{scan: bufio.NewScanner(strings.NewReader(input))}
	if _, err := s.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if len(s.pending) != 1 || s.pending[0].Text != "B" {
		t.Fatalf("pending=%+v", s.pending)
	}
}

// TestQueuedMalformed 验证先送完缓存候选，再报告下一帧解析失败。
func TestQueuedMalformed(t *testing.T) {
	input := `data: {"choices":[{"index":0,"delta":{"content":"A"}},{"index":1,"delta":{"content":"B"}}]}
data: {bad}
`
	s := &stream{scan: bufio.NewScanner(strings.NewReader(input))}
	for i := 0; i < 2; i++ {
		if _, err := s.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Next(context.Background()); err == nil {
		t.Fatal("expected malformed frame error")
	}
}
