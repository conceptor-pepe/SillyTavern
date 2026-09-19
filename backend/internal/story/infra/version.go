// version.go 冻结草稿修订，同一修订只生成一个版本。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/story/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"gorm.io/gorm"
)

// Freeze 在作品行锁内读取并冻结预期修订。
// @param ctx 请求上下文
// @param item 作品修订
// @return 版本与错误
func (r *Repo) Freeze(ctx context.Context, item domain.Story) (domain.Version, error) {
	var row model.StoryVersion
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		draft, err := lockStory(tx, item.UserID, item.ID)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if draft.Revision != item.Revision {
			return domain.ErrConflict
		}
		row, err = freezeRevision(tx, draft)
		return err
	})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.Version{}, err
	}
	return decodeVersion(row)
}

func freezeRevision(tx *gorm.DB, draft model.Story) (model.StoryVersion, error) {
	sum := sha256.Sum256([]byte(draft.Definition))
	row := model.StoryVersion{UserID: draft.UserID, StoryID: draft.ID, Revision: draft.Revision,
		Definition: draft.Definition, Digest: hex.EncodeToString(sum[:])}
	err := tx.Where("story_id = ? AND revision = ?", draft.ID, draft.Revision).FirstOrCreate(&row).Error
	return row, err
}

// Version 对外只读取未删除的本人作品版本。
// @param ctx 请求上下文
// @param uid 作者编号
// @param id 版本编号
// @return 版本与错误
func (r *Repo) Version(ctx context.Context, uid, id uint64) (domain.Version, error) {
	var row model.StoryVersion
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, uid).Where("story_id IN (?)", r.db.Model(&model.Story{}).Select("id").Where("user_id = ? AND deleted_at IS NULL", uid)).First(&row).Error
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.Version{}, mapError(err)
	}
	return decodeVersion(row)
}

func decodeVersion(row model.StoryVersion) (domain.Version, error) {
	out := domain.Version{ID: row.ID, StoryID: row.StoryID, Revision: row.Revision, Digest: row.Digest}
	err := json.Unmarshal([]byte(row.Definition), &out.Definition)
	return out, err
}
