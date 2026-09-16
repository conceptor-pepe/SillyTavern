// redis.go 适配 go-redis 进程级日志，不能在并发客户端初始化时重复安装。
package logx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"go.uber.org/zap"
)

// Redis 保存启动阶段注入的 Zap Logger。
type Redis struct{ log *zap.Logger }

// NewRedis 创建适配器，由可执行入口在创建客户端前调用 redis.SetLogger 安装。
func NewRedis(log *zap.Logger) *Redis {
	return &Redis{log: log.With(zap.String("component", "redis"))}
}

// Printf 以模板摘要定位依赖日志调用点，不展开密码、命令参数或服务器原始错误。
func (l *Redis) Printf(_ context.Context, format string, args ...any) {
	digest := sha256.Sum256([]byte(format))
	fields := append(errorFields(args), zap.String("event_id", hex.EncodeToString(digest[:8])))
	l.log.Warn("redis diagnostic", fields...)
}
