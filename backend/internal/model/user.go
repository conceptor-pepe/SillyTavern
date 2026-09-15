// user.go 定义用户和用户配置的持久化模型。
package model

// User 保存登录账号和账号状态。
type User struct {
	Base
	Handle       string `gorm:"size:64;not null;uniqueIndex"`
	Name         string `gorm:"size:128;not null"`
	PasswordHash string `gorm:"size:255;not null"`
	Enabled      bool   `gorm:"not null;default:true;index"`
	Version      int    `gorm:"not null;default:1"`
}

// UserSetting 保存用户的可扩展设置。
type UserSetting struct {
	Base
	UserID uint64 `gorm:"not null;uniqueIndex"`
	Data   string `gorm:"type:json;not null"`
}
