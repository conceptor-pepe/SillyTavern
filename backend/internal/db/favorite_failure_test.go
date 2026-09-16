// favorite_failure_test.go 验证不可自动修复的数据和索引不会被静默清理或标记升级成功。
package db

import (
	"testing"

	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
)

// TestFavoriteDuplicates 同用户重复数据阻止建唯一键，保留记录等待人工决定。
func TestFavoriteDuplicates(t *testing.T) {
	conn := testdb.Open(t)
	if err := conn.AutoMigrate(&model.Favorite{}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec("ALTER TABLE favorites DROP INDEX uk_favorites_user_char_kind").Error; err != nil {
		t.Fatal(err)
	}
	addFavorite(t, conn, 7, false)
	addFavorite(t, conn, 7, false)
	if err := Migrate(t.Context(), conn); err == nil {
		t.Fatal("duplicate data silently accepted")
	}
	checkChange(t, conn, "started")
	var count int64
	if err := conn.Model(&model.Favorite{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("failed migration lost data: count=%d err=%v", count, err)
	}
}

// TestFavoriteUnknown 未知旧索引定义必须拒绝自动删除，不猜测其业务含义。
func TestFavoriteUnknown(t *testing.T) {
	conn := testdb.Open(t)
	if err := conn.AutoMigrate(&model.Favorite{}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Exec("ALTER TABLE favorites ADD UNIQUE KEY uk_user_char_kind (character_id)").Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), conn); err == nil {
		t.Fatal("unknown legacy index silently removed")
	}
	checkChange(t, conn, "started")
	if !conn.Migrator().HasIndex(&model.Favorite{}, "uk_user_char_kind") {
		t.Fatal("unknown index lost")
	}
}
