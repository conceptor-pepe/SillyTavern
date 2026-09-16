// schema_mysql_test.go 验证版本 SQL 能从初始库升级出候选来源唯一约束。
package db

import (
	"os"
	"strings"
	"testing"

	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"gorm.io/gorm"
)

// TestSourceSchema 验证 SQL 升级既保留已有消息，又阻止一个候选生成多条有效来源记录。
func TestSourceSchema(t *testing.T) {
	conn := testdb.Open(t)
	applySQL(t, conn, "../../migrations/001_init.sql")
	row := model.Message{ConversationID: 1, Role: "assistant", Content: "original", Status: "completed", ExtraData: "{}"}
	if err := conn.Omit("source_variant_id").Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	applySQL(t, conn, "../../migrations/002_message_source.sql")
	var old model.Message
	if err := conn.First(&old, row.ID).Error; err != nil || old.Content != "original" || old.SourceVariantID != nil {
		t.Fatalf("upgrade changed old message: row=%+v err=%v", old, err)
	}
	source := uint64(7)
	for index := range 2 {
		selected := model.Message{ConversationID: 1, SourceVariantID: &source, Role: "assistant", Content: "selected", Status: "completed", ExtraData: "{}"}
		err := conn.Create(&selected).Error
		if index == 0 && err != nil {
			t.Fatal(err)
		}
		if index == 1 && err == nil {
			t.Fatal("duplicate source accepted")
		}
	}
}

// applySQL 执行仓库内固定迁移文件；当前迁移仅包含简单分号分隔的 DDL。
func applySQL(t *testing.T, conn *gorm.DB, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(content), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := conn.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
}
