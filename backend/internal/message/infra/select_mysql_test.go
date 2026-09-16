// select_mysql_test.go 使用隔离临时表验证候选选择事务和重复请求的真实数据库行为。
package infra

import (
	"context"
	"errors"
	"os"
	"testing"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestSelectionMySQL 仅在显式测试 DSN 下运行，未配置时不得冒充数据库验收。
func TestSelectionMySQL(t *testing.T) {
	dsn := os.Getenv("AI_CHAT_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("AI_CHAT_TEST_MYSQL_DSN is required for real MySQL verification")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	}()
	err = db.Connection(func(tx *gorm.DB) error {
		defer func() {
			if err := tx.Exec("DROP TEMPORARY TABLE IF EXISTS messages, conversations, message_variants").Error; err != nil {
				t.Error(err)
			}
		}()
		if err := seedSelection(tx); err != nil {
			return err
		}
		checkSelection(t, NewRepo(tx))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// seedSelection 使用临时表屏蔽同名真实表，补齐 GORM 创建和幂等索引字段。
func seedSelection(db *gorm.DB) error {
	if err := seedBranch(db); err != nil {
		return err
	}
	for _, sql := range []string{
		`ALTER TABLE messages MODIFY id BIGINT NOT NULL AUTO_INCREMENT,
		 ADD source_variant_id BIGINT NULL UNIQUE, ADD variant_no INT DEFAULT 0,
		 ADD extra_data JSON, ADD created_at DATETIME(3), ADD updated_at DATETIME(3)`,
		`CREATE TEMPORARY TABLE message_variants (id BIGINT PRIMARY KEY, message_id BIGINT,
		 variant_no INT, content TEXT, extra_data JSON, deleted_at DATETIME NULL)`,
		`INSERT INTO messages (id,parent_id,conversation_id,role,content,status,extra_data)
		 VALUES (8,3,3,'assistant','original','completed','{}')`,
		`INSERT INTO message_variants VALUES (21,8,1,'selected','{}',NULL),(22,4,1,'unrelated','{}',NULL)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

// checkSelection 验证重复请求仅创建一个独立分支，原回复保持不变。
func checkSelection(t *testing.T, repo *Repo) {
	t.Helper()
	ctx := context.Background()
	first, err := repo.SelectVariant(ctx, 7, 8, 21)
	if err != nil || first.ID == 8 || first.Content != "selected" || first.ParentID == nil || *first.ParentID != 3 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := repo.SelectVariant(ctx, 7, 8, 21)
	if err != nil || second.ID != first.ID {
		t.Fatalf("repeat=%+v err=%v", second, err)
	}
	source, err := repo.Find(ctx, 7, 8)
	if err != nil || source.Content != "original" {
		t.Fatalf("source=%+v err=%v", source, err)
	}
	for _, in := range []struct{ uid, message, variant uint64 }{{9, 8, 21}, {7, 8, 22}} {
		_, err := repo.SelectVariant(ctx, in.uid, in.message, in.variant)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("input=%+v error=%v", in, err)
		}
	}
	var count int64
	if err := repo.db.Model(&model.Message{}).Where("source_variant_id = ?", 21).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("selected messages=%d", count)
	}
	if err := repo.Delete(ctx, 7, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SelectVariant(ctx, 7, 8, 21); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("deleted selection error=%v", err)
	}
}
