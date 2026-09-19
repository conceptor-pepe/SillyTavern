// persona.go 将玩家扮演身份与登录账号资料分离。
package model

// PlayerPersona 保存用户私有人设，会话另存快照。
type PlayerPersona struct {
	Base
	UserID      uint64 `gorm:"not null;index"`
	Revision    uint64 `gorm:"not null"`
	Name        string `gorm:"size:120;not null"`
	Description string `gorm:"type:text;not null"`
	Avatar      string `gorm:"type:mediumtext;not null"`
}
