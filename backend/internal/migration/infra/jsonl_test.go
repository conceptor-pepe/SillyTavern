// jsonl_test.go 验证旧聊天 JSONL 的解析和异常边界。
package infra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestReadChat 验证消息字段和未知扩展字段都能保留。
func TestReadChat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.jsonl")
	data := "{\"role\":\"user\",\"content\":\"hello\",\"name\":\"u\",\"extra\":{\"x\":1}}\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := ReadChat(path)
	if err != nil || len(items) != 1 || items[0].Role != "user" || string(items[0].ExtraData) == "{}" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

// TestReadLegacyChat 验证真实 SillyTavern JSONL 字段映射。
func TestReadLegacyChat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.jsonl")
	data := "{\"chat_metadata\":{}}\n" +
		"{\"name\":\"角色\",\"is_user\":false,\"mes\":\"你好\"}\n" +
		"{\"name\":\"User\",\"is_user\":true,\"mes\":\"你好呀\"}\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := ReadChat(path)
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if items[0].Role != "assistant" || items[1].Role != "user" {
		t.Fatalf("roles=%q,%q", items[0].Role, items[1].Role)
	}
}

// TestExtraData 保证迁移写入器使用合法扩展 JSON，并为空值提供默认对象。
func TestExtraData(t *testing.T) {
	if got := extraData(json.RawMessage(`{"source":"legacy"}`)); got != `{"source":"legacy"}` {
		t.Fatalf("extra data=%q", got)
	}
	if got := extraData(nil); got != "{}" {
		t.Fatalf("empty extra data=%q", got)
	}
}
