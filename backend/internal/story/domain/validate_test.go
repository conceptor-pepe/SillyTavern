package domain

import (
	"strings"
	"testing"
)

func validDefinition() Definition {
	return Definition{
		SchemaVersion: 1,
		Title:         "雨夜书店",
		Cast:          []Cast{{ID: "lin", Name: "林晚"}},
		Opening: []Segment{
			{ID: "n1", Kind: "narration", Text: "雨水敲着窗。"},
			{ID: "d1", Kind: "dialogue", SpeakerID: "lin", Text: "我一直在等你。"},
		},
	}
}

// TestValidateDefinition 覆盖角色引用、内容上限和世界书触发规则。
func TestValidateDefinition(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{name: "unknown speaker", mutate: func(item *Definition) { item.Opening[1].SpeakerID = "other" }},
		{name: "duplicate segment", mutate: func(item *Definition) { item.Opening[1].ID = "n1" }},
		{name: "multiple cast", mutate: func(item *Definition) { item.Cast = append(item.Cast, Cast{ID: "two", Name: "陈舟"}) }},
		{name: "unbounded world", mutate: func(item *Definition) { item.World = strings.Repeat("x", 12001) }},
		{name: "untriggered lore", mutate: func(item *Definition) { item.Lore = []Lore{{Content: "隐藏设定"}} }},
	}
	if err := Validate(validDefinition()); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := validDefinition()
			test.mutate(&item)
			if err := Validate(item); err == nil {
				t.Fatal("invalid definition accepted")
			}
		})
	}
}

// TestOpeningText 保证兼容文本仍保留旁白和角色归属。
func TestOpeningText(t *testing.T) {
	got := OpeningText(validDefinition())
	if !strings.Contains(got, "[旁白] 雨水敲着窗。") || !strings.Contains(got, "[林晚] 我一直在等你。") {
		t.Fatalf("unexpected opening: %s", got)
	}
}
