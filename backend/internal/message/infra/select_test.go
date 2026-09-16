// select_test.go 验证候选转为独立消息时不覆盖原消息身份和父节点。
package infra

import (
	"errors"
	"testing"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// TestSelectionRow 验证新回复仅继承分支位置和候选内容。
func TestSelectionRow(t *testing.T) {
	parent := uint64(3)
	source := model.Message{Base: model.Base{ID: 8}, ConversationID: 2, ParentID: &parent, Content: "original"}
	candidate := model.MessageVariant{Base: model.Base{ID: 21}, MessageID: 8, VariantNo: 1, Content: "selected"}
	row, err := selectionRow(source, candidate)
	if err != nil || row.ID != 0 || row.ParentID == nil || *row.ParentID != parent {
		t.Fatalf("row=%+v err=%v", row, err)
	}
	if row.SourceVariantID == nil || *row.SourceVariantID != 21 || row.Content != "selected" ||
		row.Role != "assistant" || row.Status != "completed" || row.ExtraData != "{}" ||
		row.VariantNo != 1 || row.ConversationID != 2 {
		t.Fatalf("row=%+v", row)
	}
	if source.ID != 8 || source.Content != "original" {
		t.Fatalf("source changed: %+v", source)
	}
	candidate.ExtraData = "{invalid"
	if _, err := selectionRow(source, candidate); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

// TestSelectionError 验证缺失记录转为公开领域错误，数据库错误不被伪装。
func TestSelectionError(t *testing.T) {
	if !errors.Is(selectError(gorm.ErrRecordNotFound), domain.ErrNotFound) {
		t.Fatal("missing record not normalized")
	}
	cause := errors.New("database failed")
	if !errors.Is(selectError(cause), cause) {
		t.Fatal("database error lost")
	}
}
