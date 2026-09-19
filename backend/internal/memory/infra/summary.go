// summary.go 保存可重建的剧情摘要，内容散列由应用层校验。
package infra

import (
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/model"
	"context"
	"gorm.io/gorm/clause"
)

// Summaries 仅返回当前用户仍可见会话的摘要候选。
func (r *Repo) Summaries(ctx context.Context, uid, chatID uint64) ([]domain.Summary, error) {
	var rows []model.MemorySummary
	err := r.db.WithContext(ctx).Where("user_id = ? AND chat_id = ?", uid, chatID).Order("end_id DESC").Limit(200).Find(&rows).Error
	out := make([]domain.Summary, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Summary{UserID: uid, ChatID: chatID, EndID: row.EndID, Digest: row.Digest, Content: row.Content})
	}
	return out, err
}

// SaveSummary 同一前缀幂等覆盖，散列不匹配的旧结果永远不能被复用。
func (r *Repo) SaveSummary(ctx context.Context, item domain.Summary) error {
	row := model.MemorySummary{UserID: item.UserID, ChatID: item.ChatID, EndID: item.EndID, Digest: item.Digest, Content: item.Content}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "chat_id"}, {Name: "end_id"}}, DoUpdates: clause.AssignmentColumns([]string{"digest", "content", "updated_at"})}).Create(&row).Error
}
