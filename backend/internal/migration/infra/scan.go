// scan.go 负责扫描旧用户目录中的聊天 JSONL 文件。
package infra

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// ChatFile 描述一个待迁移的旧聊天文件。
type ChatFile struct {
	Path        string
	Chat        string
	Character   string
	SourcePath  string
}

// ScanChats 扫描根目录并返回所有 JSONL 聊天文件。
func ScanChats(root string) ([]ChatFile, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("migration root is required")
	}
	var files []ChatFile
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		files = append(files, ChatFile{
			Path: path, Chat: strings.TrimSuffix(filepath.Base(path), ".jsonl"),
			Character: filepath.Base(filepath.Dir(path)), SourcePath: filepath.Clean(path),
		})
		return nil
	})
	return files, err
}
