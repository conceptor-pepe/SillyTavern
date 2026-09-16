// candidates_test.go 验证交错候选聚合、完整性检查和回调错误传播。
package app

import (
	"context"
	"errors"
	"testing"

	provider "ai-chat/backend/internal/provider/domain"
)

// TestInterleavedCandidates 验证按索引组装回复且发送顺序不被排序改变。
func TestInterleavedCandidates(t *testing.T) {
	stream := &fakeStream{events: []provider.Event{
		{Type: "delta", Index: 1, Text: "B"},
		{Type: "delta", Index: 0, Text: "A"},
		{Type: "delta", Index: 1, Text: "b"},
		{Type: "delta", Index: 0, Text: "a"},
		{Type: "done"},
	}}
	var sent []int
	items, err := readCandidates(context.Background(), stream, 2, func(event provider.Event) error {
		sent = append(sent, event.Index)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Content != "Aa" || items[1].Content != "Bb" {
		t.Fatalf("items=%+v", items)
	}
	if len(sent) != 4 || sent[0] != 1 || sent[1] != 0 || sent[2] != 1 || sent[3] != 0 {
		t.Fatalf("sent=%v", sent)
	}
}

// TestInvalidCandidates 验证越界、缺失及供应商错误不能产生可保存结果。
func TestInvalidCandidates(t *testing.T) {
	cases := []struct {
		name   string
		count  int
		events []provider.Event
	}{
		{"negative count", -1, nil},
		{"negative index", 2, []provider.Event{{Type: "delta", Index: -1}}},
		{"large index", 2, []provider.Event{{Type: "delta", Index: 2}}},
		{"missing candidate", 2, []provider.Event{{Type: "delta", Text: "a"}, {Type: "done"}}},
		{"empty stream", 1, []provider.Event{{Type: "done"}}},
		{"provider error", 1, []provider.Event{{Type: "error", Error: "upstream failed"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items, err := readCandidates(context.Background(), &fakeStream{events: tc.events}, tc.count, nil)
			if err == nil || items != nil {
				t.Fatalf("items=%+v err=%v", items, err)
			}
		})
	}
}

// TestCandidateSendError 验证客户端发送失败时丢弃部分结果且保留原始错误。
func TestCandidateSendError(t *testing.T) {
	cause := errors.New("send failed")
	stream := &fakeStream{events: []provider.Event{{Type: "delta", Text: "partial"}}}
	items, err := readCandidates(context.Background(), stream, 1, func(provider.Event) error {
		return cause
	})
	if !errors.Is(err, cause) || items != nil {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}
