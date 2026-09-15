// user.go 定义旧用户存储的迁移输入结构。
package domain

// User 保存旧 KV 用户记录中可安全迁移的账号字段。
type User struct {
	Handle  string
	Name    string
	Enabled bool
}
