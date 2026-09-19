// repo_test.go 验证记忆与摘要的真实存储、隔离及幂等行为。
package infra

import (
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"testing"
)

// TestMemoryMySQL 验证不同账号、角色不能读取或覆盖记忆，摘要支持幂等替换。
func TestMemoryMySQL(t *testing.T) {
	db := testdb.Open(t)
	if err := db.AutoMigrate(&model.Character{}, &model.Memory{}, &model.MemorySummary{}); err != nil {
		t.Fatal(err)
	}
	role := model.Character{UserID: 7, Name: "A", Tags: "[]", ExtraData: "{}"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	item, err := repo.Save(t.Context(), domain.Entry{UserID: 7, CharacterID: role.ID, Kind: "fact", Content: "小鹿", Enabled: true, Pinned: true, Keywords: []string{"称呼"}, Vector: []float64{1, 0}, VectorModel: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.List(t.Context(), 8, role.ID); err == nil {
		t.Fatal("cross-user list")
	}
	if err := repo.Delete(t.Context(), 8, role.ID, item.ID); err == nil {
		t.Fatal("cross-user delete")
	}
	item.Enabled = false
	item.Content = "新名字"
	item.Vector = nil
	if _, err := repo.Save(t.Context(), item); err != nil {
		t.Fatal(err)
	}
	rows, err := repo.List(t.Context(), 7, role.ID)
	if err != nil || len(rows) != 1 || rows[0].Enabled || rows[0].Content != "新名字" || len(rows[0].Vector) != 0 {
		t.Fatal("update round trip", err)
	}
	snap := domain.Summary{UserID: 7, ChatID: 10, EndID: 100, Digest: "first", Content: "旧摘要"}
	if err := repo.SaveSummary(t.Context(), snap); err != nil {
		t.Fatal(err)
	}
	snap.Digest = "second"
	snap.Content = "新摘要"
	if err := repo.SaveSummary(t.Context(), snap); err != nil {
		t.Fatal(err)
	}
	summaries, err := repo.Summaries(t.Context(), 7, 10)
	if err != nil || len(summaries) != 1 || summaries[0].Digest != "second" {
		t.Fatal("summary idempotence", err)
	}
	if err := repo.Delete(t.Context(), 7, role.ID, item.ID); err != nil {
		t.Fatal(err)
	}
	rows, err = repo.List(t.Context(), 7, role.ID)
	if err != nil || len(rows) != 0 {
		t.Fatal("forgotten memory remains")
	}
}
