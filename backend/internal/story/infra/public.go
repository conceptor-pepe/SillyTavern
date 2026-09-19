// public.go 只投影已发布的冻结版本，草稿内容不会进入发现页。
package infra

import (
	"ai-chat/backend/internal/story/domain"
	"context"
	"encoding/json"
	"gorm.io/gorm"
	"time"
)

type publicRow struct {
	StoryID      uint64
	VersionID    uint64
	AuthorID     uint64
	AuthorName   string
	AuthorHandle string
	PublishedAt  *time.Time
	Definition   string
}

// PublicList 返回按发布时间倒序的二十条公开作品。
// @param ctx 请求上下文
// @param page 页码
// @param query 标题或标签关键词
// @return 公开作品与错误
func (r *Repo) PublicList(ctx context.Context, page int, query string) ([]domain.PublicStory, error) {
	rows := make([]publicRow, 0, 20)
	db := publicQuery(r.db.WithContext(ctx))
	if query != "" {
		db = db.Where("v.definition LIKE ?", "%"+query+"%")
	}
	if err := db.Order("s.published_at DESC, s.id DESC").Offset((page - 1) * 20).Limit(20).Scan(&rows).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	return decodePublicRows(rows)
}

// PublicFind 按作品编号读取当前公开版本。
// @param ctx 请求上下文
// @param id 作品编号
// @return 公开作品与错误
func (r *Repo) PublicFind(ctx context.Context, id uint64) (domain.PublicStory, error) {
	var row publicRow
	if err := publicQuery(r.db.WithContext(ctx)).Where("s.id = ?", id).Scan(&row).Error; err != nil {
		// audit:allow-no-log 应用层记录仓储错误。
		return domain.PublicStory{}, err
	}
	if row.StoryID == 0 {
		return domain.PublicStory{}, domain.ErrNotFound
	}
	items, err := decodePublicRows([]publicRow{row})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.PublicStory{}, err
	}
	return items[0], nil
}

func publicQuery(db *gorm.DB) *gorm.DB {
	return db.Table("stories AS s").
		Select("s.id AS story_id, v.id AS version_id, u.id AS author_id, u.name AS author_name, u.handle AS author_handle, s.published_at, v.definition").
		Joins("JOIN story_versions AS v ON v.id = s.published_version_id AND v.deleted_at IS NULL").
		Joins("JOIN users AS u ON u.id = s.user_id AND u.deleted_at IS NULL AND u.enabled = ?", true).
		Where("s.deleted_at IS NULL AND s.published_version_id IS NOT NULL")
}

func decodePublicRows(rows []publicRow) ([]domain.PublicStory, error) {
	out := make([]domain.PublicStory, 0, len(rows))
	for _, row := range rows {
		item := domain.PublicStory{StoryID: row.StoryID, VersionID: row.VersionID, AuthorID: row.AuthorID,
			AuthorName: row.AuthorName, AuthorHandle: row.AuthorHandle, PublishedAt: row.PublishedAt}
		if err := json.Unmarshal([]byte(row.Definition), &item.Definition); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return nil, err
		}
		// 写作示例与世界书只在服务端组装 Prompt，发现页不暴露作者内部提示。
		item.Definition.Examples = ""
		item.Definition.Lore = nil
		out = append(out, item)
	}
	return out, nil
}
