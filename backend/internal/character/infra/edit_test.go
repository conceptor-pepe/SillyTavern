// edit_test.go 验证角色更新保留扩展信息、清空字段及软删除归属边界。
package infra

import (
	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"strings"
	"testing"
)

// TestEditAndDeleteMySQL 覆盖真实 MySQL JSON 合并和软删除查询过滤。
func TestEditAndDeleteMySQL(t *testing.T) {
	db := testdb.Open(t)
	if err := db.AutoMigrate(&model.Character{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	item, err := repo.Create(t.Context(), domain.Character{UserID: 3, Name: "旧名", Gender: "female", Tags: []string{"旧"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.Character{}).Where("id = ?", item.ID).Update("extra_data", `{"gender":"female","unknown":"preserve"}`).Error; err != nil {
		t.Fatal(err)
	}
	other := item
	other.UserID = 4
	if _, err := repo.Update(t.Context(), other); err == nil {
		t.Fatal("cross-user update accepted")
	}
	item.Name = "新名"
	item.Gender = ""
	item.Tags = []string{}
	if _, err := repo.Update(t.Context(), item); err != nil {
		t.Fatal(err)
	}
	found, err := repo.Find(t.Context(), 3, item.ID)
	if err != nil || found.Name != "新名" || found.Gender != "" || len(found.Tags) != 0 {
		t.Fatal("update did not clear fields", err)
	}
	var row model.Character
	if err := db.First(&row, item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(row.ExtraData, "preserve") {
		t.Fatal("unknown data lost")
	}
	if err := repo.Delete(t.Context(), 4, item.ID); err == nil {
		t.Fatal("cross-user delete")
	}
	if err := repo.Delete(t.Context(), 3, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Find(t.Context(), 3, item.ID); err == nil {
		t.Fatal("deleted role readable")
	}
	if owns, err := repo.Owns(t.Context(), 3, item.ID); err != nil || owns {
		t.Fatal("deleted role owned")
	}
	_, total, err := repo.List(t.Context(), 3, 1, 20)
	if err != nil || total != 0 {
		t.Fatal("deleted role listed")
	}
	if err := db.First(&row, item.ID).Error; err != nil || row.DeletedAt == nil {
		t.Fatal("physical deletion")
	}
}
