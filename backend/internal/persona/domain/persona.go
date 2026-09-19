// persona.go 定义独立于账号资料的玩家身份。
package domain

import (
	"errors"
	"strings"
)

var ErrInvalid = errors.New("invalid persona")
var ErrNotFound = errors.New("persona not found")
var ErrConflict = errors.New("persona revision conflict")

// Persona 的内容在开聊时复制，修改不会改变旧会话。
type Persona struct {
	ID          uint64 `json:"id,string"`
	UserID      uint64 `json:"-"`
	Revision    uint64 `json:"revision,string"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Avatar      string `json:"avatar"`
}

// Validate 限制人设进入 Prompt 的大小。
// @param p 玩家人设
// @return 校验错误
func Validate(p Persona) error {
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 120 || len(p.Description) > 4000 || len(p.Avatar) > 350000 {
		return ErrInvalid
	}
	return nil
}
