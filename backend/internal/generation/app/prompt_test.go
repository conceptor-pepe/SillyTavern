// prompt_test.go 验证角色 Prompt 和历史消息的组装规则。
package app

import (
	"strings"
	"testing"

	character "ai-chat/backend/internal/character/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
)

// TestBuildPrompt 验证角色字段和历史顺序。
func TestBuildPrompt(t *testing.T) {
	got := BuildPrompt(PromptArgs{
		Character: character.Character{Name: "小满", Description: "助手", Personality: "温和"},
		History: []msgdomain.Message{
			{Role: "user", Content: "  你好  "},
			{Role: "assistant", Content: "你好呀"},
			{Role: "user", Content: " "},
		},
	})
	if len(got) != 3 || got[0].Role != "system" || got[1].Content != "  你好  " {
		t.Fatalf("unexpected prompt: %#v", got)
	}
	if got[0].Content != "角色：小满\n描述：助手\n性格：温和" {
		t.Fatalf("unexpected role prompt: %q", got[0].Content)
	}
}

// TestBuildPromptEmpty 验证空角色不会生成空 system 消息。
func TestBuildPromptEmpty(t *testing.T) {
	got := BuildPrompt(PromptArgs{History: []msgdomain.Message{{Role: "user", Content: "hi"}}})
	if len(got) != 1 || got[0].Role != "user" {
		t.Fatalf("unexpected prompt: %#v", got)
	}
}

// TestProfilePrompt 编辑后的年龄、性别和示例对话必须进入实际提示词。
func TestProfilePrompt(t *testing.T) {
	got := BuildPrompt(PromptArgs{Character: character.Character{Name: "小满", Gender: "female", Age: "24", MessageSample: "你：你好。角色：欢迎回来。", Scenario: "雨夜书店"}})
	for _, text := range []string{"性别：female", "年龄：24", "雨夜书店", "欢迎回来", "并非已发生的对话"} {
		if !strings.Contains(got[0].Content, text) {
			t.Fatalf("missing %q", text)
		}
	}
}
