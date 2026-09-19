// revise.go 在事务内把 AI 回复编辑保存为同父节点的新分支。
package infra

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReviseAssistant 锁定源回复并创建独立编辑分支。
func (r *Repo) ReviseAssistant(ctx context.Context, uid, messageID uint64, content string) (domain.Message, error) {
	var result model.Message
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source model.Message
		err := NewRepo(tx).owned(ctx, uid, messageID).Select("messages.*").
			Clauses(clause.Locking{Strength: "UPDATE"}).First(&source).Error
		if err != nil { // audit:allow-no-log 仓储错误由应用层统一记录。
			return reviseError(err)
		}
		if source.Role != "assistant" || source.Status != "completed" {
			return domain.ErrConflict
		}
		result, err = revisedRow(source, content)
		if err != nil { // audit:allow-no-log 仓储错误由应用层统一记录。
			return err
		}
		return tx.WithContext(ctx).Create(&result).Error
	})
	if err != nil { // audit:allow-no-log 仓储错误由应用层统一记录。
		return domain.Message{}, err
	}
	return toMsg(result), nil
}

// revisedRow 只继承会话和父节点，用来源编号标记审计关系。
func revisedRow(source model.Message, content string) (model.Message, error) {
	extra, err := json.Marshal(map[string]string{"edited_from": strconv.FormatUint(source.ID, 10)})
	if err != nil { // audit:allow-no-log 仓储错误由应用层统一记录。
		return model.Message{}, err
	}
	return model.Message{
		ConversationID: source.ConversationID,
		ParentID:       source.ParentID,
		Role:           "assistant",
		Content:        content,
		Status:         "completed",
		ExtraData:      string(extra),
	}, nil
}

// reviseError 将不可见消息统一映射为领域错误。
func reviseError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
