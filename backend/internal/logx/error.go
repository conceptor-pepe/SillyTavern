// error.go 将基础设施错误转换为可检索分类，禁止把驱动中的用户数据写入日志。
package logx

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/go-sql-driver/mysql"
)

// SafeError 保留错误类型和数据库编号，不保留重复键值、SQL 或连接凭据。
func SafeError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	}
	var sqlErr *mysql.MySQLError
	if errors.As(err, &sqlErr) {
		return fmt.Errorf("mysql error %d", sqlErr.Number)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.New("network timeout")
	}
	return fmt.Errorf("dependency error (%T)", err)
}
