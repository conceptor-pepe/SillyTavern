// repo.go 使用 GORM 实现用户域的数据库查询。
package infra

import (
	"context"

	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/user/domain"
	"gorm.io/gorm"
)

// UserRepo 是用户域 Repository 的 GORM 实现。
type UserRepo struct {
	db *gorm.DB
}

// NewRepo 创建用户 Repository。
func NewRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// FindHandle 按账号查询未删除用户。
func (r *UserRepo) FindHandle(ctx context.Context, handle string) (domain.User, error) {
	var row model.User
	err := r.db.WithContext(ctx).Where("handle = ?", handle).First(&row).Error
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{
		ID: row.ID, Handle: row.Handle, Name: row.Name,
		PasswordHash: row.PasswordHash, Enabled: row.Enabled, Version: row.Version,
	}, nil
}

// FindID 按用户编号查询未删除用户。
func (r *UserRepo) FindID(ctx context.Context, id uint64) (domain.User, error) {
	var row model.User
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return domain.User{}, err
	}
	return domain.User{
		ID: row.ID, Handle: row.Handle, Name: row.Name,
		PasswordHash: row.PasswordHash, Enabled: row.Enabled, Version: row.Version,
	}, nil
}

// Create 创建启用状态的用户账号。
func (r *UserRepo) Create(ctx context.Context, user domain.User) (domain.User, error) {
	row := model.User{
		Handle: user.Handle, Name: user.Name, PasswordHash: user.PasswordHash,
		Enabled: true, Version: 1,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.User{}, err
	}
	user.ID, user.Version, user.Enabled = row.ID, row.Version, row.Enabled
	return user, nil
}
