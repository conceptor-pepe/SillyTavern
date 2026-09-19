// suggest_test.go 验证回复建议的结构校验和流读取。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	provider "ai-chat/backend/internal/provider/domain"
)

// TestParseSuggestions 验证纯 JSON 和代码块都能稳定解析。
func TestParseSuggestions(t *testing.T) {
	for _, value := range []string{`["继续说","我相信你","让我想想"]`, "```json\n[\"继续说\",\"我相信你\",\"让我想想\"]\n```"} {
		items, err := parseSuggestions(value, nil)
		if err != nil || len(items) != 3 || items[1] != "我相信你" {
			t.Fatalf("items=%v err=%v", items, err)
		}
	}
}

// TestParseSuggestionsRejectsInvalid 验证数量、重复和空内容不会返回给用户。
func TestParseSuggestionsRejectsInvalid(t *testing.T) {
	for _, value := range []string{`not-json`, `["a","b"]`, `["a","a","b"]`, `["a"," ","b"]`} {
		if _, err := parseSuggestions(value, nil); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	overlong, _ := json.Marshal([]string{strings.Repeat("长", 81), "b", "c"})
	if _, err := parseSuggestions(string(overlong), nil); err == nil {
		t.Fatal("overlong suggestion accepted")
	}
}

type suggestionProvider struct {
	stream provider.Stream
	err    error
}

func (p suggestionProvider) Stream(context.Context, provider.Request) (provider.Stream, error) {
	return p.stream, p.err
}

// TestSuggest 验证模型结果被读取且不会依赖消息持久化。
func TestSuggest(t *testing.T) {
	runner := NewSuggester(suggestionProvider{stream: &fakeStream{events: []provider.Event{
		{Type: "delta", Text: `["追问","沉默","离开"]`}, {Type: "done"},
	}}}, nil)
	items, err := runner.Suggest(context.Background(), "test", []provider.Message{{Role: "assistant", Content: "你好"}}, 200)
	if err != nil || len(items) != 3 || items[2] != "离开" {
		t.Fatalf("items=%v err=%v", items, err)
	}
	broken := NewSuggester(suggestionProvider{err: errors.New("offline")}, nil)
	if _, err := broken.Suggest(context.Background(), "test", []provider.Message{{Role: "assistant", Content: "你好"}}, 200); err == nil {
		t.Fatal("provider error lost")
	}
}
