// repo.go 只读取当前作者可见的草稿。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/story/domain"
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
)

// Repo 实现作品持久化。
type Repo struct{ db *gorm.DB }

// New 创建作品仓储。
// @param db 数据连接
// @return 作品仓储
func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// OwnsCompanion 判断长期角色是否仍属于作品作者。
// @param ctx 请求上下文
// @param uid 作者编号
// @param id 角色编号
// @return 是否拥有与错误
func (r *Repo) OwnsCompanion(ctx context.Context, uid, id uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Character{}).
		Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).Count(&count).Error
	return count == 1, err
}

// List 返回固定二十条的作者作品分页。
// @param ctx 请求上下文
// @param uid 作者编号
// @param page 页码
// @return 作品与错误
func (r *Repo) List(ctx context.Context, uid uint64, page int) ([]domain.Story, error) {
	var rows []model.Story
	if err := r.db.WithContext(ctx).Where("user_id = ? AND deleted_at IS NULL", uid).Order("id DESC").Offset((page - 1) * 20).Limit(20).Find(&rows).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	out := make([]domain.Story, 0, len(rows))
	for _, row := range rows {
		item, err := decode(row)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// Find 隐藏其他作者和已删除作品。
// @param ctx 请求上下文
// @param uid 作者编号
// @param id 作品编号
// @return 草稿与错误
func (r *Repo) Find(ctx context.Context, uid, id uint64) (domain.Story, error) {
	var row model.Story
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).First(&row).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return domain.Story{}, mapError(err)
	}
	return decode(row)
}

func decode(row model.Story) (domain.Story, error) {
	item := domain.Story{ID: row.ID, UserID: row.UserID, Revision: row.Revision,
		PublishedVersionID: row.PublishedVersionID, PublishedAt: row.PublishedAt}
	err := json.Unmarshal([]byte(row.Definition), &item.Definition)
	return item, err
}

func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
