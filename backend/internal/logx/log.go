// 本文件负责创建 AI Chat 后端的结构化日志实例。
package logx

import "go.uber.org/zap"

// New 创建 JSON 格式的 Zap 日志实例，供 API 和迁移命令共用。
func New() (*zap.Logger, error) {
	return zap.NewProduction()
}
