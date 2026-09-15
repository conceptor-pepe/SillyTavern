// check.go 负责核对迁移来源统计与 MySQL 目标数据的数量、归属和关键字段。
package infra

import (
	"context"
	"errors"

	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// CheckInput 描述一次迁移校验所需的来源统计和目标用户。
type CheckInput struct {
	UserID       uint64
	SourceChats  int
	SourceMsgs   int
	SourceCards  int
	CharacterID  uint64
	CharacterIDs []uint64
}

// CheckReport 保存迁移目标校验结果。
type CheckReport struct {
	SourceChats int
	SourceMsgs  int
	SourceCards int
	TargetChats int64
	TargetMsgs  int64
	TargetCards int64
	OrphanMsgs  int64
	BadChats    int64
	BadCards    int64
	Diffs       []string
}

// Check 读取目标数据库并生成可追踪的迁移差异报告。
func (w *GormWriter) Check(ctx context.Context, in CheckInput) (CheckReport, error) {
	if in.UserID == 0 {
		return CheckReport{}, errors.New("migration check user is required")
	}
	report := CheckReport{
		SourceChats: in.SourceChats, SourceMsgs: in.SourceMsgs, SourceCards: in.SourceCards,
	}
	var err error
	report.TargetChats, err = w.countChats(ctx, in.UserID)
	if err != nil {
		return CheckReport{}, err
	}
	report.TargetMsgs, err = w.countMsgs(ctx, in.UserID)
	if err != nil {
		return CheckReport{}, err
	}
	report.TargetCards, err = w.countCards(ctx, in.UserID)
	if err != nil {
		return CheckReport{}, err
	}
	report.OrphanMsgs, err = w.countOrphans(ctx, in.UserID)
	if err != nil {
		return CheckReport{}, err
	}
	report.BadChats, err = w.countBadChats(ctx, in.UserID, characterIDs(in))
	if err != nil {
		return CheckReport{}, err
	}
	report.BadCards, err = w.countBadCards(ctx, in.UserID)
	if err != nil {
		return CheckReport{}, err
	}
	report.Diffs = diffs(report)
	return report, nil
}

// countChats 只统计目标用户未删除的会话，避免混入其他账号数据。
func (w *GormWriter) countChats(ctx context.Context, userID uint64) (int64, error) {
	return countRows(w.db.WithContext(ctx).Model(&model.Conversation{}).Where("user_id = ?", userID))
}

// countMsgs 通过会话归属统计消息，防止消息表单独出现跨用户结果。
func (w *GormWriter) countMsgs(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := w.db.WithContext(ctx).Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ?", userID).Count(&count).Error
	return count, err
}

// countCards 只统计目标用户的角色卡。
func (w *GormWriter) countCards(ctx context.Context, userID uint64) (int64, error) {
	return countRows(w.db.WithContext(ctx).Model(&model.Character{}).Where("user_id = ?", userID))
}

// countOrphans 找出没有有效会话归属的消息。
func (w *GormWriter) countOrphans(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := w.db.WithContext(ctx).Model(&model.Message{}).
		Joins("LEFT JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.id IS NULL OR conversations.user_id <> ?", userID).Count(&count).Error
	return count, err
}

// countBadChats 检查会话角色归属；零角色编号表示只检查用户归属。
func (w *GormWriter) countBadChats(ctx context.Context, userID uint64, characterIDs []uint64) (int64, error) {
	query := w.db.WithContext(ctx).Model(&model.Conversation{}).
		Where("user_id = ? AND (character_id = 0 OR title = '')", userID)
	if len(characterIDs) > 0 {
		query = query.Or("user_id = ? AND character_id NOT IN ?", userID, characterIDs)
	}
	return countRows(query)
}

// characterIDs 兼容旧的单角色校验字段，并过滤无效角色编号。
func characterIDs(in CheckInput) []uint64 {
	if len(in.CharacterIDs) > 0 {
		return in.CharacterIDs
	}
	if in.CharacterID == 0 {
		return nil
	}
	return []uint64{in.CharacterID}
}

// countBadCards 检查角色卡必须存在的用户和名称。
func (w *GormWriter) countBadCards(ctx context.Context, userID uint64) (int64, error) {
	return countRows(w.db.WithContext(ctx).Model(&model.Character{}).
		Where("user_id = ? AND (name = '' OR extra_data = '')", userID))
}

// countRows 执行统一的软删除过滤计数。
func countRows(query *gorm.DB) (int64, error) {
	var count int64
	err := query.Count(&count).Error
	return count, err
}

// diffs 将目标计数和归属异常转换成可读差异项。
func diffs(report CheckReport) []string {
	var result []string
	if int64(report.SourceChats) != report.TargetChats {
		result = append(result, "chats count mismatch")
	}
	if int64(report.SourceMsgs) != report.TargetMsgs {
		result = append(result, "messages count mismatch")
	}
	if int64(report.SourceCards) != report.TargetCards {
		result = append(result, "characters count mismatch")
	}
	if report.OrphanMsgs > 0 {
		result = append(result, "orphan messages found")
	}
	if report.BadChats > 0 {
		result = append(result, "invalid chats found")
	}
	if report.BadCards > 0 {
		result = append(result, "invalid characters found")
	}
	return result
}
