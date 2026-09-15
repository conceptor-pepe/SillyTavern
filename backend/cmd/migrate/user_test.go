// user_test.go 验证命令入口按来源目录选择用户及失败时禁止写入。
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	legacy "ai-chat/backend/internal/migration/domain"
	"go.uber.org/zap"
)

// userWriter 记录命令提交的来源用户并模拟数据库失败。
type userWriter struct {
	calls int
	item  legacy.User
	err   error
}

// ImportUser 捕获写入参数，测试不连接真实数据库。
func (w *userWriter) ImportUser(_ context.Context, item legacy.User) (bool, uint64, error) {
	w.calls++
	w.item = item
	return true, 42, w.err
}

// TestImportUser 验证目录用户不是排序首位时依然选择正确账号。
func TestImportUser(t *testing.T) {
	root := userFixture(t)
	writer := &userWriter{}
	id := importUser(context.Background(), root+string(os.PathSeparator), 0, writer, zap.NewNop())
	if id != 42 || writer.calls != 1 || writer.item.Handle != "alice" {
		t.Fatalf("id=%d writer=%+v", id, writer)
	}
	writer.err = errors.New("database failed")
	if id := importUser(context.Background(), root, 0, writer, zap.NewNop()); id != 0 {
		t.Fatalf("database failure returned user id=%d", id)
	}
}

// TestUserMatchFailure 验证缺失账号或目录布局错误不会创建任何用户。
func TestUserMatchFailure(t *testing.T) {
	root := userFixture(t)
	paths := []string{
		filepath.Join(filepath.Dir(filepath.Dir(root)), "missing", "chats"),
		filepath.Join(filepath.Dir(root), "backups"),
	}
	for _, path := range paths {
		writer := &userWriter{}
		id := importUser(context.Background(), path, 0, writer, zap.NewNop())
		if id != 0 || writer.calls != 0 {
			t.Fatalf("path=%s id=%d calls=%d", path, id, writer.calls)
		}
	}
}

// TestUserOverride 验证显式目标编号不被来源账号自动匹配替换。
func TestUserOverride(t *testing.T) {
	writer := &userWriter{}
	id := importUser(context.Background(), "no-source", 88, writer, zap.NewNop())
	if id != 88 || writer.calls != 0 {
		t.Fatalf("id=%d calls=%d", id, writer.calls)
	}
}

// userFixture 创建排序首位为其他账号的多用户存储样本。
func userFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	storage := filepath.Join(root, "_storage")
	if err := os.Mkdir(storage, 0700); err != nil {
		t.Fatal(err)
	}
	rows := []string{
		`{"key":"user:bob","value":{"handle":"bob"}}`,
		`{"key":"user:alice","value":{"handle":"alice","name":"Alice"}}`,
	}
	for index, row := range rows {
		path := filepath.Join(storage, []string{"a", "b"}[index])
		if err := os.WriteFile(path, []byte(row), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, "alice", "chats")
}
