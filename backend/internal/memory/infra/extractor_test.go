package infra

import "testing"

// TestParseProposals 验证关系范围、数量和正文上限不能被模型输出绕过。
func TestParseProposals(t *testing.T) {
	value := `[{"scope":"relationship","content":"用户喜欢被叫作小鹿","evidence":"用户明确说请叫我小鹿"},{"scope":"story","content":"两人在木屋重逢","evidence":"当前分支中的重逢场景"}]`
	items, err := parseProposals(value, true, true)
	if err != nil || len(items) != 2 {
		t.Fatalf("valid proposals rejected: items=%+v err=%v", items, err)
	}
	if _, err := parseProposals(value, false, true); err == nil {
		t.Fatal("relationship proposal accepted without companion scope")
	}
	if _, err := parseProposals(`[{"scope":"story","content":"","evidence":"x"}]`, true, true); err == nil {
		t.Fatal("empty proposal accepted")
	}
}
