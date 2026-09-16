// card.go 负责读取 PNG tEXt 块中的 SillyTavern 角色卡。
package infra

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"

	"ai-chat/backend/internal/migration/domain"
)

// cardFields 映射 SillyTavern 角色卡的标准字段名。
type cardFields struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Personality   string   `json:"personality"`
	Scenario      string   `json:"scenario"`
	FirstMessage  string   `json:"first_mes"`
	MessageSample string   `json:"mes_example"`
	Creator       string   `json:"creator"`
	CreatorNotes  string   `json:"creator_notes"`
	Avatar        string   `json:"avatar"`
	Tags          []string `json:"tags"`
}

// ReadCard 读取 PNG 内的 chara 或 ccv3 角色卡数据。
func ReadCard(path string) (domain.Character, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Character{}, err
	}
	for _, key := range []string{"ccv3", "chara"} {
		value, ok := textChunk(data, key)
		if ok {
			return parseCard(value)
		}
	}
	return domain.Character{}, errors.New("character card not found")
}

// textChunk 查找指定名称的 PNG 文本块。
func textChunk(data []byte, want string) ([]byte, bool) {
	for pos := 8; pos+12 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		end := pos + 12 + size
		if end > len(data) {
			return nil, false
		}
		kind := string(data[pos+4 : pos+8])
		if kind == "tEXt" {
			body := data[pos+8 : pos+8+size]
			parts := bytes.SplitN(body, []byte{0}, 2)
			if len(parts) == 2 && string(parts[0]) == want {
				return parts[1], true
			}
		}
		pos = end
	}
	return nil, false
}

// parseCard 解码角色卡，并兼容字段位于根节点或 data 节点的版本。
func parseCard(value []byte) (domain.Character, error) {
	raw, err := base64.StdEncoding.DecodeString(string(value))
	if err != nil {
		return domain.Character{}, err
	}
	var root cardFields
	var card struct {
		Data cardFields `json:"data"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		// audit:allow-no-log 由迁移命令统一记录角色卡路径和解析错误。
		return domain.Character{}, err
	}
	if err := json.Unmarshal(raw, &card); err != nil {
		return domain.Character{}, err
	}
	mergeFields(&root, card.Data)
	return toLegacy(root, raw), nil
}

// mergeFields 使用新版 data 节点补齐旧版根节点缺失的字段。
func mergeFields(dst *cardFields, src cardFields) {
	fillText(&dst.Name, src.Name)
	fillText(&dst.Description, src.Description)
	fillText(&dst.Personality, src.Personality)
	fillText(&dst.Scenario, src.Scenario)
	fillText(&dst.FirstMessage, src.FirstMessage)
	fillText(&dst.MessageSample, src.MessageSample)
	fillText(&dst.Creator, src.Creator)
	fillText(&dst.CreatorNotes, src.CreatorNotes)
	fillText(&dst.Avatar, src.Avatar)
	if len(dst.Tags) == 0 {
		dst.Tags = src.Tags
	}
}

// fillText 仅用非空来源补齐空字段。
func fillText(dst *string, src string) {
	if *dst == "" {
		*dst = src
	}
}

// toLegacy 转换为迁移域角色并保留原始卡片数据。
func toLegacy(card cardFields, raw []byte) domain.Character {
	return domain.Character{
		Name: card.Name, Description: card.Description, Personality: card.Personality,
		Scenario: card.Scenario, FirstMessage: card.FirstMessage,
		MessageSample: card.MessageSample, Creator: card.Creator,
		CreatorNotes: card.CreatorNotes, Tags: card.Tags, Avatar: card.Avatar, ExtraData: raw,
	}
}
