// publish.go 以冻结版本作为唯一公共入口，后续草稿编辑不会改变已发布内容。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/story/domain"
	"context"
	"gorm.io/gorm"
	"time"
)

// Publish 在作品锁内冻结预期修订并切换公开指针。
// @param ctx 请求上下文
// @param item 作品及预期修订
// @return 更新后的作品与错误
func (r *Repo) Publish(ctx context.Context, item domain.Story) (domain.Story, error) {
	var row model.Story
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = lockStory(tx, item.UserID, item.ID)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if row.Revision != item.Revision {
			return domain.ErrConflict
		}
		version, err := freezeRevision(tx, row)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		now := time.Now().UTC()
		row.PublishedVersionID, row.PublishedAt = &version.ID, &now
		return tx.Model(&row).Updates(map[string]any{
			"published_version_id": version.ID, "published_at": now,
		}).Error
	})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.Story{}, err
	}
	return decode(row)
}

// Unpublish 清除公开指针但保留所有不可变版本和既有会话。
// @param ctx 请求上下文
// @param uid 作者编号
// @param id 作品编号
// @return 更新后的作品与错误
func (r *Repo) Unpublish(ctx context.Context, uid, id uint64) (domain.Story, error) {
	var row model.Story
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = lockStory(tx, uid, id)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		row.PublishedVersionID, row.PublishedAt = nil, nil
		return tx.Model(&row).Updates(map[string]any{
			"published_version_id": nil, "published_at": nil,
		}).Error
	})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.Story{}, err
	}
	return decode(row)
}
