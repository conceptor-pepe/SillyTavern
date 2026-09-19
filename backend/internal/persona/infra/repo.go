// repo.go 将人设归属固定在用户范围内。
package infra

import (
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/persona/domain"
	"context"
	"errors"
	"gorm.io/gorm"
)

// Repo 保存人设数据库连接。
type Repo struct{ db *gorm.DB }

// New 创建人设仓储。
// @param db 数据连接
// @return 人设仓储
func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// List 返回二十条身份记录，避免无限制加载头像。
// @param ctx 请求上下文
// @param uid 用户编号
// @param page 页码
// @return 人设与错误
func (r *Repo) List(ctx context.Context, uid uint64, page int) ([]domain.Persona, error) {
	var rows []model.PlayerPersona
	err := r.db.WithContext(ctx).Where("user_id = ? AND deleted_at IS NULL", uid).Order("id DESC").Offset((page - 1) * 20).Limit(20).Find(&rows).Error
	out := make([]domain.Persona, 0, len(rows))
	for _, row := range rows {
		out = append(out, convert(row))
	}
	return out, err
}

// Find 不区分不存在与跨账号的人设。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 人设编号
// @return 人设与错误
func (r *Repo) Find(ctx context.Context, uid, id uint64) (domain.Persona, error) {
	var row model.PlayerPersona
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).First(&row).Error
	return convert(row), mapError(err)
}

func convert(row model.PlayerPersona) domain.Persona {
	return domain.Persona{ID: row.ID, UserID: row.UserID, Revision: row.Revision, Name: row.Name, Description: row.Description, Avatar: row.Avatar}
}

func mapError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
