// favorite_mysql_test.go 验证真实 MySQL 收藏索引升级、重复执行、结构漂移及数据保留。
package db

import (
	"testing"

	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"gorm.io/gorm"
)

// TestFavoriteUpgrade 覆盖空库、SQL 初始库和旧 GORM 库三种升级来源。
func TestFavoriteUpgrade(t *testing.T) {
	for _, source := range []string{"empty", "sql", "gorm"} {
		t.Run(source, func(t *testing.T) {
			conn := testdb.Open(t)
			if source == "sql" {
				applySQL(t, conn, "../../migrations/001_init.sql")
			}
			if source == "gorm" {
				legacyFavorites(t, conn)
			}
			if source != "empty" {
				addFavorite(t, conn, 7, false)
			}
			for range 2 {
				if err := Migrate(t.Context(), conn); err != nil {
					t.Fatal(err)
				}
			}
			if source == "empty" {
				addFavorite(t, conn, 7, false)
			}
			addFavorite(t, conn, 8, false)
			addFavorite(t, conn, 7, true)
			checkFavoriteRows(t, conn)
			checkChange(t, conn, "applied")
		})
	}
}

// legacyFavorites 重现历史 GORM 两列唯一索引，升级必须移除此跨用户限制。
func legacyFavorites(t *testing.T, conn *gorm.DB) {
	t.Helper()
	if err := conn.AutoMigrate(&model.Favorite{}); err != nil {
		t.Fatal(err)
	}
	err := conn.Exec(`ALTER TABLE favorites DROP INDEX uk_favorites_user_char_kind,
		ADD UNIQUE KEY uk_user_char_kind (character_id, kind)`).Error
	if err != nil {
		t.Fatal(err)
	}
}

// addFavorite 同一对象允许不同用户收藏，同一用户重复写入必须由数据库拒绝。
func addFavorite(t *testing.T, conn *gorm.DB, uid uint64, duplicate bool) {
	t.Helper()
	row := model.Favorite{UserID: uid, CharacterID: 3, Kind: "character"}
	err := conn.Create(&row).Error
	if (err != nil) != duplicate {
		t.Fatalf("favorite uid=%d duplicate=%v err=%v", uid, duplicate, err)
	}
}

// checkFavoriteRows 确认升级没有删除或改变已有收藏归属。
func checkFavoriteRows(t *testing.T, conn *gorm.DB) {
	t.Helper()
	var rows []model.Favorite
	if err := conn.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].UserID != 7 || rows[1].UserID != 8 ||
		rows[0].CharacterID != 3 || rows[1].Kind != "character" {
		t.Fatalf("favorites changed: %+v", rows)
	}
}

// checkChange 只有目标结构已经校验的升级才允许记录 applied。
func checkChange(t *testing.T, conn *gorm.DB, state string) {
	t.Helper()
	var rows []schemaChange
	if err := conn.Find(&rows).Error; err != nil || len(rows) != 1 {
		t.Fatalf("changes=%+v err=%v", rows, err)
	}
	if rows[0].Version != favoriteVersion || rows[0].State != state || rows[0].Digest == "" {
		t.Fatalf("invalid change: %+v", rows)
	}
}

// TestFavoriteDrift 已完成升级的索引漂移或版本摘要变化必须阻止继续启动。
func TestFavoriteDrift(t *testing.T) {
	for _, change := range []string{
		"ALTER TABLE favorites DROP INDEX uk_favorites_user_char_kind",
		"UPDATE schema_changes SET digest = 'changed'",
		"UPDATE schema_changes SET state = 'unknown'",
	} {
		t.Run(change, func(t *testing.T) {
			conn := testdb.Open(t)
			if err := Migrate(t.Context(), conn); err != nil {
				t.Fatal(err)
			}
			if err := conn.Exec(change).Error; err != nil {
				t.Fatal(err)
			}
			if err := Migrate(t.Context(), conn); err == nil {
				t.Fatal("schema drift accepted")
			}
		})
	}
}

// TestFavoriteResume 模拟 DDL 提交前后中断，started 记录可通过真实结构检查继续升级。
func TestFavoriteResume(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(map[bool]string{false: "before DDL", true: "after DDL"}[applied], func(t *testing.T) {
			conn := testdb.Open(t)
			legacyFavorites(t, conn)
			if err := prepareChange(conn); err != nil {
				t.Fatal(err)
			}
			if applied {
				err := conn.Exec(`ALTER TABLE favorites DROP INDEX uk_user_char_kind,
					ADD UNIQUE KEY uk_favorites_user_char_kind (user_id, character_id, kind)`).Error
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := Migrate(t.Context(), conn); err != nil {
				t.Fatal(err)
			}
			checkChange(t, conn, "applied")
		})
	}
}
