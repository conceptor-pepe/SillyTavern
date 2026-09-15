// import_test.go 验证角色批量迁移的统计和错误隔离。
package app

import (
	"context"
	"errors"
	"testing"

	legacy "ai-chat/backend/internal/migration/domain"
)

type fakeCards struct {
	seen map[string]bool
	fail string
}

func (f *fakeCards) ImportCharacter(_ context.Context, _ uint64, item legacy.Character, path string) (bool, error) {
	if path == f.fail {
		return false, errors.New("write failed")
	}
	if f.seen[item.Name] {
		return false, nil
	}
	f.seen[item.Name] = true
	return true, nil
}

// TestImportCharacters 验证创建、跳过和失败记录分别统计。
func TestImportCharacters(t *testing.T) {
	files := []string{"a.png", "b.png", "bad.png"}
	reader := func(path string) (legacy.Character, error) {
		if path == "bad.png" {
			return legacy.Character{}, errors.New("read failed")
		}
		return legacy.Character{Name: "same"}, nil
	}
	writer := &fakeCards{seen: map[string]bool{}, fail: ""}
	created, skipped, failures := ImportCharacters(context.Background(), 1, files, reader, writer)
	if created != 1 || skipped != 1 || len(failures) != 1 {
		t.Fatalf("created=%d skipped=%d failures=%v", created, skipped, failures)
	}
}
