// branch_test.go 在每次测试独立创建的 MySQL 库中验证递归查询，不修改业务数据。
package infra

import (
	"errors"
	"testing"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/testdb"
	"gorm.io/gorm"
)

// TestBranchMySQL 使用隔离普通表测试递归语义，避免 MySQL 临时表的重复引用限制。
func TestBranchMySQL(t *testing.T) {
	db := testdb.Open(t)
	if err := seedBranch(db.WithContext(t.Context())); err != nil {
		t.Fatal(err)
	}
	checkBranchRows(t, NewRepo(db))
}

// seedBranch 在当前连接中建立隔离数据，覆盖兄弟分支和软删除记录。
func seedBranch(db *gorm.DB) error {
	for _, sql := range []string{
		`CREATE TABLE conversations (id BIGINT PRIMARY KEY, user_id BIGINT, deleted_at DATETIME NULL)`,
		`CREATE TABLE messages (id BIGINT PRIMARY KEY, parent_id BIGINT NULL,
		 conversation_id BIGINT, role VARCHAR(32), content TEXT, status VARCHAR(32), deleted_at DATETIME NULL)`,
		`INSERT INTO conversations VALUES (3,7,NULL),(4,8,NULL),(5,7,NOW())`,
		`INSERT INTO messages VALUES
		 (1,NULL,3,'user','root','completed',NULL),
		 (2,1,3,'assistant','selected','completed',NULL),
		 (3,2,3,'user','question','completed',NULL),
		 (4,1,3,'assistant','sibling','completed',NULL),
		 (5,NULL,4,'user','private','completed',NULL),
		 (6,NULL,3,'user','deleted','completed',NOW()),
		 (7,NULL,5,'user','deleted chat','completed',NULL)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

// checkBranchRows 断言父链有序且不包含兄弟、他人或已删除记录。
func checkBranchRows(t *testing.T, repo *Repo) {
	t.Helper()
	items, err := repo.Branch(t.Context(), 7, 3, 3)
	if err != nil || len(items) != 3 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	for i, item := range items {
		if item.ID != uint64(i+1) {
			t.Fatalf("unexpected order: %v", items)
		}
	}
	for _, in := range []struct{ uid, chatID, id uint64 }{
		{8, 3, 3}, {7, 4, 5}, {7, 3, 5}, {7, 3, 6}, {7, 5, 7}, {7, 3, 99},
	} {
		_, err := repo.Branch(t.Context(), in.uid, in.chatID, in.id)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("input=%+v err=%v", in, err)
		}
	}
}
