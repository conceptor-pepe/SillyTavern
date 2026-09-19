// revise_test.go 验证 AI 编辑分支只继承必要的剧情位置。
package infra

import (
	"strings"
	"testing"

	"ai-chat/backend/internal/model"
)

// TestRevisedRow 验证编辑不会覆盖源消息或沿用候选身份。
func TestRevisedRow(t *testing.T) {
	parent := uint64(4)
	variant := uint64(9)
	source := model.Message{Base: model.Base{ID: 8}, ConversationID: 2, ParentID: &parent,
		SourceVariantID: &variant, Role: "assistant", Content: "old", Status: "completed", ExtraData: "{}"}
	row, err := revisedRow(source, "new")
	if err != nil {
		t.Fatal(err)
	}
	if row.ID != 0 || row.ConversationID != 2 || row.ParentID == nil || *row.ParentID != parent ||
		row.SourceVariantID != nil || row.Role != "assistant" || row.Content != "new" || row.Status != "completed" {
		t.Fatalf("row=%+v", row)
	}
	if !strings.Contains(row.ExtraData, `"edited_from":"8"`) || source.Content != "old" {
		t.Fatalf("row=%+v source=%+v", row, source)
	}
}
