// asset.go 定义用户上传资源的持久化模型。
package model

// Asset 保存文件逻辑信息和原始存储路径。
type Asset struct {
	Base
	UserID    uint64 `gorm:"not null;index"`
	Name      string `gorm:"size:255;not null"`
	Path      string `gorm:"size:512;not null"`
	Mime      string `gorm:"size:128;not null"`
	Size      int64  `gorm:"not null;default:0"`
	ExtraData string `gorm:"type:json;not null"`
}
