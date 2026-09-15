// character.go 定义旧角色卡迁移输入结构。
package domain

// Character 保存旧 PNG 角色卡中的核心字段和原始扩展数据。
type Character struct {
	Name          string
	Description   string
	Personality   string
	Scenario      string
	FirstMessage  string
	MessageSample string
	Creator       string
	Tags          []string
	ExtraData     []byte
	Avatar        string
}
