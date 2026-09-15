// register.go 负责用户注册用例。
package app

import (
	"context"
	"errors"
	"strings"

	"ai-chat/backend/internal/user/domain"
)

// RegisterIn 保存用户注册参数。
type RegisterIn struct {
	Handle   string
	Name     string
	Password string
}

// Register 创建一个新的用户账号。
func Register(ctx context.Context, repo domain.Repo, in RegisterIn) (LoginOut, error) {
	if strings.TrimSpace(in.Handle) == "" || strings.TrimSpace(in.Password) == "" {
		return LoginOut{}, errors.New("invalid register input")
	}
	hash, err := HashPass(in.Password)
	if err != nil {
		return LoginOut{}, err
	}
	user, err := repo.Create(ctx, domain.User{
		Handle: in.Handle, Name: in.Name, PasswordHash: hash, Enabled: true,
	})
	if err != nil {
		return LoginOut{}, err
	}
	return LoginOut{ID: user.ID, Handle: user.Handle, Name: user.Name, Version: user.Version}, nil
}
