// background.go 保存账号级聊天背景，所有设备共享同一份配置。
package model

// Background 保存经过重新编码的图片，不允许任意远程 URL。
type Background struct {
	Base
	UserID  uint64 `gorm:"not null;uniqueIndex"`
	Image   string `gorm:"type:mediumtext;not null"`
	Enabled bool   `gorm:"not null"`
}
