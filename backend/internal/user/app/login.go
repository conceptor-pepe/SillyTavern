// login.go 负责用户登录用例，不处理 HTTP 和数据库细节。
package app

import (
	"context"

	"ai-chat/backend/internal/user/domain"
)

// LoginIn 保存登录用例的输入参数。
type LoginIn struct {
	Handle   string
	Password string
}

// LoginOut 保存登录成功后的用户信息。
type LoginOut struct {
	ID      uint64
	Handle  string
	Name    string
	Version int
}

// Login 执行用户查询、状态检查和密码校验。
func Login(ctx context.Context, repo domain.Repo, in LoginIn, match func(string, string) bool) (LoginOut, error) {
	user, err := repo.FindHandle(ctx, in.Handle)
	if err != nil {
		return LoginOut{}, err
	}
	if err := user.CheckLogin(in.Password, match); err != nil {
		return LoginOut{}, err
	}
	return LoginOut{ID: user.ID, Handle: user.Handle, Name: user.Name, Version: user.Version}, nil
}
