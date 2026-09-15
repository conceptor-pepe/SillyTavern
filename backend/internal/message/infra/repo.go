// repo.go 使用 GORM 实现消息历史查询。
package infra

import (
	"context"
	"encoding/json"
	"errors"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// Repo 是消息 Repository 的 GORM 实现。
type Repo struct{ db *gorm.DB }

// NewRepo 创建消息 Repository。
func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// List 查询指定用户会话中的消息。
func (r *Repo) List(ctx context.Context, uid, chatID uint64, page, size int) ([]domain.Message, int64, error) {
	var rows []model.Message
	var total int64
	query := r.db.WithContext(ctx).Where("conversation_id = ?", chatID)
	if err := query.Model(&model.Message{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset((page - 1) * size).Limit(size).Order("id ASC").Find(&rows).Error
	return mapMsgs(rows), total, err
}

// Create 保存消息，用户归属由应用层在调用前完成校验。
func (r *Repo) Create(ctx context.Context, uid uint64, item domain.Message) (domain.Message, error) {
	row := model.Message{
		ConversationID: item.ConversationID, ParentID: item.ParentID, Role: item.Role,
		Content: item.Content, Status: item.Status, VariantNo: item.VariantNo,
		ExtraData: extraData(item.ExtraData),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Message{}, err
	}
	return toMsg(row), nil
}

// Find 查询当前用户可见的一条消息。
func (r *Repo) Find(ctx context.Context, uid, messageID uint64) (domain.Message, error) {
	var row model.Message
	err := r.owned(ctx, uid, messageID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Message{}, errors.New("message not found")
	}
	return toMsg(row), err
}

// OwnsInChat 校验父消息属于当前用户和指定会话。
func (r *Repo) OwnsInChat(ctx context.Context, uid, chatID, messageID uint64) (bool, error) {
	var count int64
	err := r.owned(ctx, uid, messageID).Where("messages.conversation_id = ?", chatID).Count(&count).Error
	return count == 1, err
}

// Update 只允许修改用户消息，并通过会话用户归属限制目标。
func (r *Repo) Update(ctx context.Context, uid, messageID uint64, content string) (domain.Message, error) {
	var row model.Message
	err := r.owned(ctx, uid, messageID).First(&row).Error
	if err != nil {
		return domain.Message{}, err
	}
	if row.Role != "user" {
		return domain.Message{}, errors.New("only user message can be edited")
	}
	if err := r.db.WithContext(ctx).Model(&row).Update("content", content).Error; err != nil {
		return domain.Message{}, err
	}
	row.Content = content
	return toMsg(row), nil
}

// Delete 软删除属于当前用户会话的消息，避免物理删除破坏历史审计。
func (r *Repo) Delete(ctx context.Context, uid, messageID uint64) error {
	var row model.Message
	if err := r.owned(ctx, uid, messageID).First(&row).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&row).Error
}

// owned 通过会话连接同时校验消息存在和消息所属用户。
func (r *Repo) owned(ctx context.Context, uid, messageID uint64) *gorm.DB {
	return r.db.WithContext(ctx).Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("messages.id = ? AND conversations.user_id = ?", messageID, uid)
}

// mapMsgs 转换消息列表，避免暴露 GORM 模型。
func mapMsgs(rows []model.Message) []domain.Message {
	out := make([]domain.Message, 0, len(rows))
	for _, row := range rows {
		out = append(out, toMsg(row))
	}
	return out
}

// toMsg 转换单条消息。
func toMsg(row model.Message) domain.Message {
	return domain.Message{
		ID: row.ID, ConversationID: row.ConversationID, ParentID: row.ParentID,
		Role: row.Role, Content: row.Content, Status: row.Status,
		VariantNo: row.VariantNo, ExtraData: []byte(row.ExtraData),
	}
}

// extraData 统一校验消息扩展字段，避免非法 JSON 破坏数据库约束。
func extraData(value json.RawMessage) string {
	if len(value) == 0 || !json.Valid(value) {
		return "{}"
	}
	return string(value)
}
