// redis.go 负责创建 Redis 客户端连接。
package db

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// OpenRedis 创建 Redis 客户端并检查连接。
func OpenRedis(ctx context.Context, addr, pass string, index int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       index,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		closeErr := client.Close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return client, nil
}
