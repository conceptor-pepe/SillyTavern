// user.go 负责读取旧版用户 KV 文件。
package infra

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-chat/backend/internal/migration/domain"
)

// ListUsers 返回旧用户 KV 文件列表。
func ListUsers(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			paths = append(paths, filepath.Join(root, entry.Name()))
		}
	}
	return paths, nil
}

// ReadUser 从旧 KV 文件读取用户账号。
func ReadUser(path string) (domain.User, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.User{}, err
	}
	var row struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &row); err != nil {
		return domain.User{}, err
	}
	if !strings.HasPrefix(row.Key, "user:") {
		return domain.User{}, domain.ErrSkip
	}
	var item domain.User
	if err := json.Unmarshal(row.Value, &item); err != nil {
		return domain.User{}, err
	}
	if item.Handle == "" || row.Key != "user:"+item.Handle {
		return domain.User{}, errors.New("legacy user key and handle mismatch")
	}
	return item, nil
}

// FindUser 精确匹配来源账号，歧义或损坏的存储不得降级为首个用户。
func FindUser(root, handle string) (domain.User, error) {
	if handle == "" {
		return domain.User{}, errors.New("legacy user handle is required")
	}
	paths, err := ListUsers(root)
	if err != nil {
		return domain.User{}, err
	}
	var found domain.User
	for _, path := range paths {
		item, err := ReadUser(path)
		if errors.Is(err, domain.ErrSkip) {
			continue
		}
		if err != nil {
			return domain.User{}, fmt.Errorf("read legacy user %s: %w", path, err)
		}
		if item.Handle != handle {
			continue
		}
		if found.Handle != "" {
			return domain.User{}, errors.New("duplicate legacy user handle")
		}
		found = item
	}
	if found.Handle == "" {
		return domain.User{}, fmt.Errorf("legacy user %s: %w", handle, domain.ErrMissing)
	}
	return found, nil
}
