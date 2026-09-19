// edit.go 在事务内更新角色，保留旧角色卡的未知扩展字段。
package infra

import (
	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/model"
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Update 锁定当前用户角色并更新完整资料，空字段也允许清除。
func (r *Repo) Update(ctx context.Context, item domain.Character) (domain.Character, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.Character
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", item.ID, item.UserID).First(&row).Error; err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return err
		}
		// 空值也必须写入，不能让旧值在 JSON 合并后复活。
		extra, err := json.Marshal(struct {
			Portrait string `json:"portrait"`
			Gender   string `json:"gender"`
			Age      string `json:"age"`
		}{item.Portrait, item.Gender, item.Age})
		if err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return err
		}
		tags, err := json.Marshal(item.Tags)
		if err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return err
		}
		patch := model.Character{Name: item.Name, Description: item.Description, Personality: item.Personality, Scenario: item.Scenario, FirstMessage: item.FirstMessage, MessageSample: item.MessageSample, Tags: string(tags)}
		if err := tx.Model(&row).Select("Name", "Description", "Personality", "Scenario", "FirstMessage", "MessageSample", "Tags").Updates(patch).Error; err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return err
		}
		return tx.Model(&row).Update("extra_data", gorm.Expr("JSON_MERGE_PATCH(extra_data, ?)", string(extra))).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return item, domain.ErrNotFound
	}
	return item, err
}

// Delete 归属与软删除条件在同一语句检查，历史消息不被物理删除。
func (r *Repo) Delete(ctx context.Context, uid, id uint64) error {
	out := r.db.WithContext(ctx).Model(&model.Character{}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).Update("deleted_at", time.Now().UTC())
	if out.Error != nil {
		return out.Error
	}
	if out.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
