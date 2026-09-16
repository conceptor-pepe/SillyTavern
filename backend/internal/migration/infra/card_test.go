// card_test.go 验证 PNG 角色卡读取器能解析仓库中的真实角色资源。
package infra

import (
	"path/filepath"
	"testing"

	legacy "ai-chat/backend/internal/migration/domain"
	"ai-chat/backend/internal/model"
)

// TestReadCard 验证真实角色卡的标准下划线字段可完整读取。
func TestReadCard(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "data", "default-user", "characters", "default_Seraphina.png")
	card, err := ReadCard(path)
	if err != nil {
		t.Fatal(err)
	}
	if card.Name != "Seraphina" || card.Description == "" || card.FirstMessage == "" {
		t.Fatalf("card fields missing: %+v", card)
	}
	if card.CreatorNotes == "" || len(card.ExtraData) == 0 {
		t.Fatalf("card=%+v", card)
	}
}

// TestMissingFields 验证只补齐迁移角色的空字段。
func TestMissingFields(t *testing.T) {
	fields, err := missingFields(model.Character{Description: "已编辑", Tags: "[]"}, legacy.Character{
		Description: "旧描述", FirstMessage: "你好", CreatorNotes: "备注", Tags: []string{"内置"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if fields["description"] != nil || fields["first_message"] != "你好" || fields["creator_notes"] != "备注" {
		t.Fatalf("fields=%v", fields)
	}
	if fields["tags"] != `["内置"]` {
		t.Fatalf("tags=%v", fields["tags"])
	}
}
