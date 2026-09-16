// select.go 在同一事务内锁定原消息和候选，通过唯一来源创建独立回复分支。
package infra

import (
	"context"
	"errors"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SelectVariant 锁定源消息以串行化重复选择，失败不返回未提交的消息编号。
func (r *Repo) SelectVariant(ctx context.Context, uid, messageID, variantID uint64) (domain.Message, error) {
	var result model.Message
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = NewRepo(tx).selectReply(ctx, uid, messageID, variantID)
		return err
	})
	if err != nil {
		return domain.Message{}, err
	}
	return toMsg(result), nil
}

// selectReply 先锁源消息再锁候选，同一候选重复选择返回已有分支。
func (r *Repo) selectReply(ctx context.Context, uid, messageID, variantID uint64) (model.Message, error) {
	var source model.Message
	err := r.owned(ctx, uid, messageID).Select("messages.*").
		Clauses(clause.Locking{Strength: "UPDATE"}).First(&source).Error
	if err != nil {
		return model.Message{}, selectError(err)
	}
	if source.Role != "assistant" || source.Status != "completed" {
		return model.Message{}, domain.ErrConflict
	}
	var candidate model.MessageVariant
	err = r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND message_id = ? AND deleted_at IS NULL", variantID, messageID).First(&candidate).Error
	if err != nil {
		return model.Message{}, selectError(err)
	}
	return r.saveSelection(ctx, source, candidate)
}

// saveSelection 使用来源唯一键保持幂等，已删除的选中分支不被隐式恢复。
func (r *Repo) saveSelection(ctx context.Context, source model.Message, candidate model.MessageVariant) (model.Message, error) {
	var row model.Message
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("source_variant_id = ?", candidate.ID).First(&row).Error
	if err == nil {
		if row.DeletedAt != nil || row.ConversationID != source.ConversationID {
			return model.Message{}, domain.ErrConflict
		}
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Message{}, err
	}
	row, err = selectionRow(source, candidate)
	if err != nil {
		return model.Message{}, err
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return model.Message{}, err
	}
	return row, nil
}

// selectionRow 保留父节点与候选内容，但不复用原消息主键和时间字段。
func selectionRow(source model.Message, candidate model.MessageVariant) (model.Message, error) {
	extra, err := extraData([]byte(candidate.ExtraData))
	if err != nil {
		return model.Message{}, err
	}
	return model.Message{
		ConversationID: source.ConversationID, ParentID: source.ParentID, SourceVariantID: &candidate.ID,
		Role: "assistant", Status: "completed", Content: candidate.Content,
		VariantNo: candidate.VariantNo, ExtraData: extra,
	}, nil
}

// selectError 屏蔽缺失源消息与候选之间的差异，保留真实存储错误供日志记录。
func selectError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
