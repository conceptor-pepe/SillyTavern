// mysql.go 负责创建 GORM MySQL 数据库连接。
package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"ai-chat/backend/internal/logx"
	driver "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// OpenMySQL 根据 DSN 创建数据库连接并设置连接池。
func OpenMySQL(ctx context.Context, dsn string, log *zap.Logger) (*gorm.DB, error) {
	sqlDB, err := openPool(dsn, log)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, errors.Join(err, sqlDB.Close())
	}
	conn, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logx.NewGorm(log), DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, errors.Join(err, sqlDB.Close())
	}
	return conn, nil
}

// openPool 通过结构化 DSN 配置注入驱动日志，避免进程级可变设置。
func openPool(dsn string, log *zap.Logger) (*sql.DB, error) {
	config, err := driver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	config.Logger = logx.NewMySQL(log)
	connector, err := driver.NewConnector(config)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}
