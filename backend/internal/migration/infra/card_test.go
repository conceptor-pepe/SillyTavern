// card_test.go 验证 PNG 角色卡读取器能解析仓库中的真实角色资源。
package infra

import (
	"path/filepath"
	"testing"
)

// TestReadCard 验证真实角色卡的名称和核心字段可读取。
func TestReadCard(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "data", "default-user", "characters", "临夏.png")
	card, err := ReadCard(path)
	if err != nil {
		t.Fatal(err)
	}
	if card.Name == "" || card.Description == "" || len(card.ExtraData) == 0 {
		t.Fatalf("card=%+v", card)
	}
}
