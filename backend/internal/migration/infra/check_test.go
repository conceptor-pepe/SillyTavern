// check_test.go 验证迁移差异报告对数量和归属异常的识别。
package infra

import "testing"

// TestCharacterIDs 验证多角色集合优先于旧单角色字段。
func TestCharacterIDs(t *testing.T) {
	got := characterIDs(CheckInput{CharacterID: 3, CharacterIDs: []uint64{4, 5}})
	if len(got) != 2 || got[0] != 4 || got[1] != 5 {
		t.Fatalf("character ids=%v", got)
	}
	got = characterIDs(CheckInput{CharacterID: 3})
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("legacy character ids=%v", got)
	}
}

// TestDiffs 验证来源与目标不一致时输出对应差异项。
func TestDiffs(t *testing.T) {
	report := CheckReport{
		SourceChats: 2, TargetChats: 1,
		SourceMsgs: 4, TargetMsgs: 3,
		SourceCards: 1, TargetCards: 0,
		OrphanMsgs: 1, BadChats: 2, BadCards: 1,
	}
	diffs := diffs(report)
	if len(diffs) != 6 {
		t.Fatalf("diffs=%v", diffs)
	}
}

// TestDiffsClean 验证数量和关键字段全部一致时报告为空。
func TestDiffsClean(t *testing.T) {
	report := CheckReport{
		SourceChats: 2, TargetChats: 2,
		SourceMsgs: 4, TargetMsgs: 4,
		SourceCards: 1, TargetCards: 1,
	}
	if got := diffs(report); len(got) != 0 {
		t.Fatalf("unexpected diffs=%v", got)
	}
}
