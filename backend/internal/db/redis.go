// redis.go 负责创建 Redis 客户端连接。
package db

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

// OpenRedis 创建 Redis 客户端并检查连接。
func OpenRedis(ctx context.Context, addr, pass string, index int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       index,
		// 当前部署固定为 Redis 7，不发送 Redis 8 的维护通知握手。
		MaintNotificationsConfig: &maintnotifications.Config{Mode: maintnotifications.ModeDisabled},
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
