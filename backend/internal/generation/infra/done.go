// done.go 负责在一个 GORM 事务中保存 assistant 消息并完成生成任务。
package infra

import (
	"context"
	"errors"

	msgdomain "ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// DoneWriter 是生成完成事务写入器。
type DoneWriter struct {
	db *gorm.DB
}

// NewDoneWriter 创建生成完成事务写入器。
func NewDoneWriter(db *gorm.DB) *DoneWriter {
	return &DoneWriter{db: db}
}

// Create 保存非事务兼容路径使用的消息。
func (w *DoneWriter) Create(ctx context.Context, userID uint64, item msgdomain.Message) (msgdomain.Message, error) {
	row := model.Message{
		ConversationID: item.ConversationID, ParentID: item.ParentID,
		Role: item.Role, Content: item.Content, Status: item.Status,
		VariantNo: item.VariantNo, ExtraData: "{}",
	}
	if err := w.db.WithContext(ctx).Create(&row).Error; err != nil {
		return msgdomain.Message{}, err
	}
	return msgdomain.Message{
		ID: row.ID, ConversationID: row.ConversationID, ParentID: row.ParentID,
		Role: row.Role, Content: row.Content, Status: row.Status,
		VariantNo: row.VariantNo, ExtraData: []byte(row.ExtraData),
	}, nil
}

// SaveDone 在同一事务中保存 assistant 消息并更新 generation。
func (w *DoneWriter) SaveDone(ctx context.Context, userID, generationID uint64, item msgdomain.Message, finished int64) (msgdomain.Message, error) {
	var out msgdomain.Message
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := model.Message{
			ConversationID: item.ConversationID, ParentID: item.ParentID,
			Role: item.Role, Content: item.Content, Status: item.Status,
			VariantNo: item.VariantNo, ExtraData: "{}",
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		variants, err := saveCandidates(tx, row.ID, item.Variants)
		if err != nil {
			return err
		}
		result := tx.Model(&model.Generation{}).
			Where("user_id = ? AND id = ? AND conversation_id = ? AND status = ?",
				userID, generationID, item.ConversationID, "running").
			Updates(map[string]any{"status": "completed", "message_id": row.ID, "finished_at": finished})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("generation is not running")
		}
		out = msgdomain.Message{
			ID: row.ID, ConversationID: row.ConversationID, ParentID: row.ParentID,
			Role: row.Role, Content: row.Content, Status: row.Status,
			VariantNo: row.VariantNo, ExtraData: []byte(row.ExtraData),
			Variants: variants,
		}
		return nil
	})
	if err != nil {
		return msgdomain.Message{}, err
	}
	return out, err
}
