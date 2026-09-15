// base.go 定义所有持久化模型共用的时间字段。
package model

import "time"

// Base 保存记录的创建、更新时间和软删除时间。
type Base struct {
	ID        uint64     `gorm:"primaryKey"`
	CreatedAt time.Time  `gorm:"not null"`
	UpdatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `gorm:"index"`
}
