// user.go 定义用户实体和登录相关的领域规则。
package domain

import (
	"errors"
	"strings"
)

// 用户域错误用于区分登录失败的业务原因。
var (
	ErrBadHandle = errors.New("invalid handle")
	ErrDisabled  = errors.New("user disabled")
	ErrPassword  = errors.New("invalid password")
)

// User 表示用户域中的账号实体。
type User struct {
	ID           uint64
	Handle       string
	Name         string
	PasswordHash string
	Enabled      bool
	Version      int
}

// CheckLogin 校验账号状态和登录所需字段。
func (u User) CheckLogin(password string, match func(string, string) bool) error {
	if strings.TrimSpace(u.Handle) == "" {
		return ErrBadHandle
	}
	if !u.Enabled {
		return ErrDisabled
	}
	if !match(password, u.PasswordHash) {
		return ErrPassword
	}
	return nil
}
