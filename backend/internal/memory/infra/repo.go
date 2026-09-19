// repo.go 在角色归属范围内保存记忆，硬删除条目使用户遗忘操作立即生效。
package infra

import (
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/model"
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 保存记忆存储连接。
type Repo struct{ db *gorm.DB }

// NewRepo 创建记忆仓储。
func NewRepo(db *gorm.DB) *Repo { return &Repo{db: db} }

// owned 统一校验角色仍存在且属于当前账号。
func (r *Repo) owned(ctx context.Context, uid, charID uint64) error {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Character{}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", charID, uid).Count(&count).Error
	if err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return err
	}
	if count != 1 {
		return domain.ErrNotFound
	}
	return nil
}

// List 角色记忆限制为二百条，避免加载无限大的检索集。
func (r *Repo) List(ctx context.Context, uid, charID uint64) ([]domain.Entry, error) {
	if err := r.owned(ctx, uid, charID); err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return nil, err
	}
	return r.listScope(ctx, uid, charID)
}

func (r *Repo) listScope(ctx context.Context, uid, charID uint64) ([]domain.Entry, error) {
	var rows []model.Memory
	err := r.db.WithContext(ctx).Where("user_id = ? AND character_id = ? AND chat_id = 0", uid, charID).Order("id DESC").Limit(201).Find(&rows).Error
	if err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return nil, err
	}
	items := make([]domain.Entry, 0, len(rows))
	for _, row := range rows {
		item, err := readEntry(row)
		if err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// readEntry 损坏的 JSON 明确报错，不能悄悄把记忆当成空列表。
func readEntry(row model.Memory) (domain.Entry, error) {
	item := domain.Entry{ID: row.ID, UserID: row.UserID, ChatID: row.ChatID, CharacterID: row.CharacterID, Kind: row.Kind, Content: row.Content, Enabled: row.Enabled, Pinned: row.Pinned, VectorModel: row.EmbeddingModel}
	if err := json.Unmarshal([]byte(row.Keywords), &item.Keywords); err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return item, err
	}
	err := json.Unmarshal([]byte(row.Embedding), &item.Vector)
	return item, err
}

// Save 锁定角色以串行检查配额，更新空字段不会遗漏 disabled 状态。
func (r *Repo) Save(ctx context.Context, item domain.Entry) (domain.Entry, error) {
	if item.ChatID != 0 {
		return r.saveChat(ctx, item)
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var char model.Character
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", item.CharacterID, item.UserID).First(&char).Error; err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return domain.ErrNotFound
		}
		row, err := entryRow(item)
		if err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return err
		}
		query := tx.Model(&model.Memory{}).Where("user_id = ? AND character_id = ?", item.UserID, item.CharacterID)
		if item.ID != 0 {
			return updateEntry(query, row)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return err
		}
		if count >= 200 {
			return domain.ErrInvalid
		}
		if err := tx.Create(&row).Error; err != nil {
			// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
			return err
		}
		item.ID = row.ID
		return nil
	})
	return item, err
}

// entryRow 将可选向量以 JSON 编码，未配置服务时保存空数组。
func entryRow(item domain.Entry) (model.Memory, error) {
	keys, err := json.Marshal(item.Keywords)
	if err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return model.Memory{}, err
	}
	vector, err := json.Marshal(item.Vector)
	return model.Memory{Base: model.Base{ID: item.ID}, UserID: item.UserID, ChatID: item.ChatID, CharacterID: item.CharacterID, Kind: item.Kind, Content: item.Content, Keywords: string(keys), Enabled: item.Enabled, Pinned: item.Pinned, Embedding: string(vector), EmbeddingModel: item.VectorModel}, err
}

// updateEntry 禁止 PUT 不存在的编号隐式创建新记录。
func updateEntry(query *gorm.DB, row model.Memory) error {
	var previous model.Memory
	if err := query.Where("id = ?", row.ID).First(&previous).Error; err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return domain.ErrNotFound
	}
	return query.Where("id = ?", row.ID).Select("kind", "content", "keywords", "enabled", "pinned", "embedding", "embedding_model").Updates(row).Error
}

// Delete 立即移除正文与向量，后续请求不能再召回。
func (r *Repo) Delete(ctx context.Context, uid, charID, id uint64) error {
	if err := r.owned(ctx, uid, charID); err != nil {
		// audit:allow-no-log 错误由应用服务或调用方统一记录，仓储/供应商不重复记录内容。
		return err
	}
	out := r.db.WithContext(ctx).Unscoped().Where("id = ? AND user_id = ? AND character_id = ?", id, uid, charID).Delete(&model.Memory{})
	if out.Error != nil {
		return out.Error
	}
	if out.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
