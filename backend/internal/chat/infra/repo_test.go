// repo_test.go 检查会话作用域与删除语句，避免误将时间指针当作 GORM 自动软删除。
package infra

import (
	"strings"
	"testing"

	"ai-chat/backend/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestChatScope 确认读取与删除均带用户边界和删除过滤，删除只更新记录。
func TestChatScope(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(localhost:3306)/unused", SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	query := repo.visible(t.Context(), 7).Where("id = ?", 3).Find(&model.Conversation{})
	for _, part := range []string{"SELECT", "user_id = ?", "deleted_at IS NULL", "id = ?"} {
		if !strings.Contains(query.Statement.SQL.String(), part) {
			t.Errorf("missing %q: %s", part, query.Statement.SQL.String())
		}
	}
	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return NewRepo(tx).visible(t.Context(), 7).Where("id = ?", 3).Update("deleted_at", "2026-09-16")
	})
	if !strings.HasPrefix(sql, "UPDATE") || !strings.Contains(sql, "deleted_at IS NULL") {
		t.Fatalf("unsafe delete SQL: %s", sql)
	}
}
