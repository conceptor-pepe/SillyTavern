// profile_test.go 验证迁移资料兼容与真实数据库保存后的展示字段一致性。
package infra

import (
	"reflect"
	"testing"

	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
)

// TestBuiltinPortrait 只按迁移来源匹配内置图片，同名自建角色不冒用封面。
func TestBuiltinPortrait(t *testing.T) {
	item := toChar(model.Character{Name: "Seraphina", Tags: "[]", ExtraData: `{"source_path":"characters/default_Seraphina.png"}`})
	if item.Portrait != "/img/characters/seraphina.jpg" {
		t.Fatal("builtin cover missing")
	}
	custom := toChar(model.Character{Name: "Seraphina", Tags: "[]", ExtraData: "{}"})
	if custom.Portrait != "" {
		t.Fatal("custom role used builtin cover")
	}
}

// TestProfileMySQL 保存后重新查询，验证 JSON 展示字段与用户归属均未丢失。
func TestProfileMySQL(t *testing.T) {
	db := testdb.Open(t)
	if err := db.AutoMigrate(&model.Character{}); err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	input := domain.Character{UserID: 7, Name: "角色", Tags: []string{"日常", "冒险"},
		Portrait: "data:image/jpeg;base64,fixture", Gender: "female", Age: "24", MessageSample: "示例"}
	saved, err := repo.Create(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	input.ID = saved.ID
	found, err := repo.Find(t.Context(), 7, saved.ID)
	if err != nil || !reflect.DeepEqual(found, input) {
		t.Fatalf("round trip mismatch: %v", err)
	}
	if _, err := repo.Find(t.Context(), 8, saved.ID); err == nil {
		t.Fatal("cross-user read accepted")
	}
	items, total, err := repo.List(t.Context(), 7, 1, 20)
	if err != nil || total != 1 || !reflect.DeepEqual(items[0], input) {
		t.Fatal("list round trip mismatch")
	}
}
