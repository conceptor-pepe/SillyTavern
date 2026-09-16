// variants_test.go 验证候选查询生成的 SQL 含完整可见性条件，不代替真实数据库验证。
package infra

import (
	"context"
	"slices"
	"strings"
	"testing"

	"ai-chat/backend/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestVariantScope 验证归属、三层删除状态和候选列选择均进入 SQL。
func TestVariantScope(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(localhost:3306)/unused", SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	var rows []model.MessageVariant
	query := NewRepo(db).variants(context.Background(), 7, 12).
		Select("message_variants.*").Order("message_variants.variant_no ASC, message_variants.id ASC").
		Offset(20).Limit(20).Find(&rows)
	if query.Error != nil {
		t.Fatal(query.Error)
	}
	sql := query.Statement.SQL.String()
	for _, part := range []string{
		"message_variants.*", "messages.id = ?", "conversations.user_id = ?",
		"messages.deleted_at IS NULL", "conversations.deleted_at IS NULL",
		"message_variants.deleted_at IS NULL", "LIMIT ? OFFSET ?",
		"ORDER BY message_variants.variant_no ASC, message_variants.id ASC",
	} {
		if !strings.Contains(sql, part) {
			t.Errorf("missing %q in %s", part, sql)
		}
	}
	if !slices.Equal(query.Statement.Vars, []any{uint64(7), uint64(12), 20, 20}) {
		t.Fatalf("vars=%v", query.Statement.Vars)
	}
}
