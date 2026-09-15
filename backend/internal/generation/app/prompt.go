// prompt.go 负责把角色资料和聊天历史组装成 Provider 上下文。
package app

import (
	"strings"

	character "ai-chat/backend/internal/character/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
)

// PromptArgs 保存 Prompt 组装所需的角色和历史消息。
type PromptArgs struct {
	Character character.Character
	History   []msgdomain.Message
}

// BuildPrompt 生成包含角色设定和历史消息的 Provider 请求上下文。
func BuildPrompt(args PromptArgs) []provider.Message {
	items := make([]provider.Message, 0, len(args.History)+1)
	if text := roleText(args.Character); text != "" {
		items = append(items, provider.Message{Role: "system", Content: text})
	}
	for _, item := range args.History {
		if strings.TrimSpace(item.Content) == "" || item.Role == "" {
			continue
		}
		items = append(items, provider.Message{Role: item.Role, Content: item.Content})
	}
	return items
}

// roleText 保持角色字段顺序稳定，便于模型理解和测试。
func roleText(item character.Character) string {
	parts := make([]string, 0, 5)
	addPart(&parts, "角色", item.Name)
	addPart(&parts, "描述", item.Description)
	addPart(&parts, "性格", item.Personality)
	addPart(&parts, "场景", item.Scenario)
	addPart(&parts, "开场白", item.FirstMessage)
	return strings.Join(parts, "\n")
}

// addPart 忽略空字段，避免生成多余的标签和空白。
func addPart(parts *[]string, name, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		*parts = append(*parts, name+"："+value)
	}
}
