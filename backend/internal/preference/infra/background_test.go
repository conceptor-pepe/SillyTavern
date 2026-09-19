// background_test.go 验证账号背景可回读、清空与用户隔离。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/preference/domain"
	"ai-chat/backend/internal/testdb"
	"testing"
)

// TestBackgroundMySQL 相同账号不同仓储实例能读取同一背景，另一个账号默认空白。
func TestBackgroundMySQL(t *testing.T) {
	db := testdb.Open(t)
	if err := db.AutoMigrate(&model.Background{}); err != nil {
		t.Fatal(err)
	}
	a, b := New(db), New(db)
	if err := a.Save(t.Context(), 1, domain.Background{Image: "image", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	item, err := b.Get(t.Context(), 1)
	if err != nil || item.Image != "image" || !item.Enabled {
		t.Fatal("cross-device read", err)
	}
	other, err := b.Get(t.Context(), 2)
	if err != nil || other.Image != "" || other.Enabled {
		t.Fatal("cross-user leak", err)
	}
	if err := b.Save(t.Context(), 1, domain.Background{}); err != nil {
		t.Fatal(err)
	}
	item, err = a.Get(t.Context(), 1)
	if err != nil || item.Image != "" || item.Enabled {
		t.Fatal("clear failed", err)
	}
}
