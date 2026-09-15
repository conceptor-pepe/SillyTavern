// user_test.go 验证旧用户 KV 文件的读取。
package infra

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ai-chat/backend/internal/migration/domain"
)

// TestReadUser 验证账号、名称和启用状态映射正确。
func TestReadUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user")
	data := `{"key":"user:default-user","value":{"handle":"default-user","name":"User","enabled":true}}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	item, err := ReadUser(path)
	if err != nil || item.Handle != "default-user" || !item.Enabled {
		t.Fatalf("item=%+v err=%v", item, err)
	}
}

// TestReadUserKeys 验证非用户记录不会冒充用户，键与账号必须一致。
func TestReadUserKeys(t *testing.T) {
	cases := []struct {
		name string
		data string
		skip bool
	}{
		{"avatar", `{"key":"avatar:alice","value":"image"}`, true},
		{"other", `{"key":"other","value":{"handle":"alice"}}`, true},
		{"mismatch", `{"key":"user:bob","value":{"handle":"alice"}}`, false},
		{"empty", `{"key":"user:","value":{}}`, false},
		{"broken", `{"key":`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeUserKV(t, t.TempDir(), "kv", tc.data)
			_, err := ReadUser(path)
			if err == nil || errors.Is(err, domain.ErrSkip) != tc.skip {
				t.Fatalf("error=%v skip=%v", err, tc.skip)
			}
		})
	}
}

// TestFindUser 验证账号匹配不依赖文件排序，且不会启用旧禁用账号。
func TestFindUser(t *testing.T) {
	root := t.TempDir()
	writeUserKV(t, root, "0", `{"key":"avatar:alice","value":"image"}`)
	writeUserKV(t, root, "1", `{"key":"user:bob","value":{"handle":"bob"}}`)
	writeUserKV(t, root, "2", `{"key":"user:alice","value":{"handle":"alice","enabled":false}}`)
	item, err := FindUser(root, "alice")
	if err != nil || item.Handle != "alice" || item.Enabled {
		t.Fatalf("item=%+v err=%v", item, err)
	}
	_, err = FindUser(root, "missing")
	if !errors.Is(err, domain.ErrMissing) {
		t.Fatalf("missing error=%v", err)
	}
	writeUserKV(t, root, "3", `{"key":"user:alice","value":{"handle":"alice"}}`)
	if _, err := FindUser(root, "alice"); err == nil {
		t.Fatal("duplicate user accepted")
	}
}

// TestFindUserErrors 验证损坏文件及缺失存储不允许自动匹配。
func TestFindUserErrors(t *testing.T) {
	root := t.TempDir()
	writeUserKV(t, root, "0", `{"key":"user:alice","value":{"handle":"alice"}}`)
	writeUserKV(t, root, "1", `{`)
	if _, err := FindUser(root, "alice"); err == nil {
		t.Fatal("corrupt storage accepted")
	}
	if _, err := FindUser(root, ""); err == nil {
		t.Fatal("empty handle accepted")
	}
	if _, err := FindUser(filepath.Join(root, "missing"), "alice"); err == nil {
		t.Fatal("missing storage accepted")
	}
}

// writeUserKV 创建隔离 KV 样本，不读取或改写真实用户数据。
func writeUserKV(t *testing.T, root, name, data string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
