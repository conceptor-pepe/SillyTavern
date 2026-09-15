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

// ReadCard 读取 PNG 内的 chara 或 ccv3 角色卡数据。
func ReadCard(path string) (domain.Character, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.Character{}, err
	}
	for _, key := range []string{"ccv3", "chara"} {
		value, ok := textChunk(data, key)
		if !ok {
			continue
		}
		return parseCard(value)
	}
	return domain.Character{}, errors.New("character card not found")
}

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

func parseCard(value []byte) (domain.Character, error) {
	raw, err := base64.StdEncoding.DecodeString(string(value))
	if err != nil {
		return domain.Character{}, err
	}
	var card struct {
		Name, Description, Personality, Scenario, FirstMessage, MessageSample string
		Creator, CreatorNotes, Avatar                                         string
		Tags                                                                  []string
		Data                                                                  struct {
			Name, Description, Personality, Scenario, FirstMessage, MessageSample string
			Creator, CreatorNotes, Avatar                                         string
			Tags                                                                  []string
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &card); err != nil {
		return domain.Character{}, err
	}
	applyData(&card)
	return domain.Character{
		Name: card.Name, Description: card.Description, Personality: card.Personality,
		Scenario: card.Scenario, FirstMessage: card.FirstMessage,
		MessageSample: card.MessageSample, Creator: card.Creator, Tags: card.Tags,
		Avatar: card.Avatar, ExtraData: raw,
	}, nil
}

func applyData(card *struct {
	Name, Description, Personality, Scenario, FirstMessage, MessageSample string
	Creator, CreatorNotes, Avatar                                         string
	Tags                                                                  []string
	Data                                                                  struct {
		Name, Description, Personality, Scenario, FirstMessage, MessageSample string
		Creator, CreatorNotes, Avatar                                         string
		Tags                                                                  []string
	} `json:"data"`
}) {
	if card.Name == "" {
		card.Name = card.Data.Name
	}
	if card.Description == "" {
		card.Description = card.Data.Description
	}
	if card.Personality == "" {
		card.Personality = card.Data.Personality
	}
	if card.Scenario == "" {
		card.Scenario = card.Data.Scenario
	}
	if card.FirstMessage == "" {
		card.FirstMessage = card.Data.FirstMessage
	}
	if card.MessageSample == "" {
		card.MessageSample = card.Data.MessageSample
	}
	if card.Creator == "" {
		card.Creator = card.Data.Creator
	}
	if len(card.Tags) == 0 {
		card.Tags = card.Data.Tags
	}
	if card.Avatar == "" {
		card.Avatar = card.Data.Avatar
	}
}
