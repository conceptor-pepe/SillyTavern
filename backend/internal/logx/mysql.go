// mysql.go 适配 MySQL 驱动连接级日志，避免绕过 GORM 输出未脱敏错误。
package logx

import "go.uber.org/zap"

// MySQL 保存连接池专用的日志实例。
type MySQL struct{ log *zap.Logger }

// NewMySQL 为底层驱动创建独立适配器，不修改驱动全局 Logger。
func NewMySQL(log *zap.Logger) *MySQL {
	return &MySQL{log: log.With(zap.String("component", "mysql"))}
}

// Print 仅保留错误分类，丢弃连接字符串和服务器返回的任意文本。
func (l *MySQL) Print(args ...any) {
	l.log.Warn("mysql driver diagnostic", errorFields(args)...)
}
