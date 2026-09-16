// create_test.go 验证角色创建的输入校验和字段清理。
package app

import (
	"context"
	"testing"

	"ai-chat/backend/internal/character/domain"
)

type fakeCreator struct{ item domain.Character }

// Create 保存测试角色并返回。
func (f *fakeCreator) Create(_ context.Context, item domain.Character) (domain.Character, error) {
	f.item = item
	item.ID = 9
	return item, nil
}

// TestCreate 验证角色名称必填且资料会去除首尾空白。
func TestCreate(t *testing.T) {
	repo := &fakeCreator{}
	item, err := createItem(t, repo, domain.Character{
		UserID: 3, Name: "  小林  ", Description: "  书店  ",
	})
	if err != nil || item.ID != 9 || repo.item.Name != "小林" || repo.item.Description != "书店" {
		t.Fatalf("item=%+v saved=%+v err=%v", item, repo.item, err)
	}
}

// TestCreateRejectsEmpty 验证空名称不会写入数据库。
func TestCreateRejectsEmpty(t *testing.T) {
	repo := &fakeCreator{}
	_, err := createItem(t, repo, domain.Character{UserID: 3})
	if err != domain.ErrInvalid {
		t.Fatalf("err=%v", err)
	}
	if repo.item.ID != 0 {
		t.Fatalf("unexpected write: %+v", repo.item)
	}
}

// createItem 使用测试上下文调用角色创建服务。
func createItem(t *testing.T, repo domain.CreatorRepo, item domain.Character) (domain.Character, error) {
	t.Helper()
	return NewCreator(repo).Create(t.Context(), item)
}
