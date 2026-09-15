// 本文件负责创建 AI Chat 后端的结构化日志实例。
package logx

import "go.uber.org/zap"

// New 创建开发环境使用的 Zap 日志实例。
func New() (*zap.Logger, error) {
	return zap.NewDevelopment()
}
