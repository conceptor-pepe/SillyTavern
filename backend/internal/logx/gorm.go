// gorm.go 将 GORM 日志接入 Zap，不记录 SQL、绑定参数或驱动错误原文。
package logx

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm/logger"
)

// Gorm 实现连接级日志适配器；日志级别副本不共享可变状态。
type Gorm struct {
	log   *zap.Logger
	level logger.LogLevel
}

// NewGorm 默认记录错误和超过两百毫秒的慢查询。
func NewGorm(log *zap.Logger) *Gorm {
	return &Gorm{log: log.With(zap.String("component", "gorm")), level: logger.Warn}
}

// LogMode 返回独立配置副本，避免 Debug 查询影响并发请求。
func (l *Gorm) LogMode(level logger.LogLevel) logger.Interface {
	copy := *l
	copy.level = level
	return &copy
}

// Info 仅记录框架事件分类，不展开可能包含隐私的消息模板。
func (l *Gorm) Info(_ context.Context, _ string, _ ...any) {
	if l.level >= logger.Info {
		l.log.Info("gorm notice")
	}
}

// Warn 仅记录框架警告分类，不展开驱动参数。
func (l *Gorm) Warn(_ context.Context, _ string, _ ...any) {
	if l.level >= logger.Warn {
		l.log.Warn("gorm warning")
	}
}

// Error 隔离 GORM 初始化等错误参数，业务入口负责记录操作上下文。
func (l *Gorm) Error(_ context.Context, _ string, args ...any) {
	if l.level >= logger.Error {
		l.log.Error("gorm error", errorFields(args)...)
	}
}

// Trace 不调用 SQL 展开回调，即使开启 Debug 也不生成或记录带参数的语句。
func (l *Gorm) Trace(_ context.Context, begin time.Time, _ func() (string, int64), err error) {
	if l.level == logger.Silent || errors.Is(err, logger.ErrRecordNotFound) {
		return
	}
	elapsed := time.Since(begin)
	fields := []zap.Field{zap.Int64("duration_ms", elapsed.Milliseconds())}
	switch {
	case err != nil && l.level >= logger.Error:
		l.log.Error("database query failed", append(fields, zap.Error(SafeError(err)))...)
	case elapsed >= 200*time.Millisecond && l.level >= logger.Warn:
		l.log.Warn("database query slow", fields...)
	case l.level >= logger.Info:
		l.log.Info("database query completed", fields...)
	}
}

// errorFields 只取参数中的首个安全错误分类，忽略文本、SQL 和任意对象。
func errorFields(args []any) []zap.Field {
	for _, arg := range args {
		if err, ok := arg.(error); ok {
			return []zap.Field{zap.Error(SafeError(err))}
		}
	}
	return []zap.Field{zap.Error(errors.New("dependency error"))}
}
