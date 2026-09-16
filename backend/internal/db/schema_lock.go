// schema_lock.go 在固定 MySQL 连接持有升级锁，防止多个 API 或导入进程同时修改结构。
package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// schemaLock 将锁与 DDL 固定到同一连接；请求取消也必须独立释放锁。
func schemaLock(ctx context.Context, conn *gorm.DB, run func(*gorm.DB) error) error {
	return conn.WithContext(ctx).Connection(func(bound *gorm.DB) (result error) {
		// 固定连接不等于复用 Statement，原始 SQL 的目标不能污染后续模型查询。
		bound = bound.Session(&gorm.Session{NewDB: true})
		var database string
		if err := bound.Raw("SELECT DATABASE()").Scan(&database).Error; err != nil {
			return err
		}
		if database == "" {
			return errors.New("schema database required")
		}
		name := schemaKey(database)
		var acquired sql.NullInt64
		err := bound.Raw("SELECT GET_LOCK(?, 30)", name).Scan(&acquired).Error
		if err != nil {
			return errors.Join(err, discardSchema(bound))
		}
		if !acquired.Valid || acquired.Int64 != 1 {
			return errors.New("schema lock unavailable")
		}
		defer func() { result = errors.Join(result, releaseSchema(bound, name)) }()
		return run(bound)
	})
}

// releaseSchema 释放失败时丢弃物理连接，禁止带着会话锁返回连接池。
func releaseSchema(conn *gorm.DB, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var released sql.NullInt64
	err := conn.WithContext(ctx).Raw("SELECT RELEASE_LOCK(?)", name).Scan(&released).Error
	if err == nil && released.Valid && released.Int64 == 1 {
		return nil
	}
	failure := errors.Join(errors.New("schema lock release failed"), err)
	return errors.Join(failure, discardSchema(conn))
}

// discardSchema 锁结果不确定时关闭物理连接，不能只将连接还回池中。
func discardSchema(conn *gorm.DB) error {
	if physical, ok := conn.Statement.ConnPool.(*sql.Conn); ok {
		return physical.Raw(func(any) error { return driver.ErrBadConn })
	}
	return errors.New("schema connection is not pinned")
}

// schemaKey 将当前库名称散列为固定长度锁名，避免不同库之间争抢同一升级锁。
func schemaKey(database string) string {
	return fmt.Sprintf("ai_chat_schema_%x", sha256.Sum256([]byte(database)))[:64]
}
