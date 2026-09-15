// jsonl.go 负责只读解析旧版聊天 JSONL 文件。
package infra

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"

	"ai-chat/backend/internal/migration/domain"
)

// ReadChat 读取 JSONL 文件并保留未识别字段。
func ReadChat(path string) ([]domain.Message, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readLines(file)
}

func readLines(file *os.File) ([]domain.Message, error) {
	var result []domain.Message
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		item, err := parseLine(scanner.Bytes())
		if err != nil {
			if errors.Is(err, domain.ErrSkip) {
				continue
			}
			return nil, err
		}
		result = append(result, item)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func parseLine(data []byte) (domain.Message, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return domain.Message{}, err
	}
	if _, ok := raw["chat_metadata"]; ok {
		return domain.Message{}, domain.ErrSkip
	}
	role, err := readText(raw, "role")
	if err != nil {
		return domain.Message{}, err
	}
	content, err := readText(raw, "content")
	if err != nil {
		return domain.Message{}, err
	}
	if content == "" {
		content, err = readText(raw, "mes")
		if err != nil {
			return domain.Message{}, err
		}
	}
	if role == "" {
		role, err = readRole(raw)
		if err != nil {
			return domain.Message{}, err
		}
	}
	name, err := readText(raw, "name")
	if err != nil {
		return domain.Message{}, err
	}
	extra, err := unknown(raw)
	if err != nil {
		return domain.Message{}, err
	}
	item := domain.Message{Role: role, Content: content, Name: name, ExtraData: extra}
	if item.Role == "" || item.Content == "" {
		return domain.Message{}, errors.New("legacy message requires role and content")
	}
	return item, nil
}

// readRole 根据旧版 is_user 字段转换统一角色名。
func readRole(raw map[string]json.RawMessage) (string, error) {
	var isUser bool
	data, ok := raw["is_user"]
	if !ok {
		return "", nil
	}
	if err := json.Unmarshal(data, &isUser); err != nil {
		return "", err
	}
	if isUser {
		return "user", nil
	}
	return "assistant", nil
}

func readText(raw map[string]json.RawMessage, key string) (string, error) {
	var value string
	if data, ok := raw[key]; ok {
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
	}
	return value, nil
}

func unknown(raw map[string]json.RawMessage) (json.RawMessage, error) {
	for _, key := range []string{"role", "content", "name"} {
		delete(raw, key)
	}
	return json.Marshal(raw)
}
