// scan_test.go 验证旧用户目录的聊天文件扫描边界。
package infra

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanChats 验证只收集 JSONL 文件并保留聊天名称。
func TestScanChats(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.jsonl"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.txt"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	files, err := ScanChats(root)
	if err != nil || len(files) != 1 || files[0].Chat != "one" {
		t.Fatalf("files=%+v err=%v", files, err)
	}
}
