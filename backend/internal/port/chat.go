// chat.go 定义会话域向其他业务域提供的可见会话事务能力。
package port

import "context"

// ChatGate 在当前用户的可见会话锁内执行操作，回调错误必须回滚同一事务。
// 实现通过上下文传递事务，领域接口不暴露 GORM 或私有数据表。
type ChatGate interface {
	WithChat(ctx context.Context, userID, chatID uint64, run func(context.Context) error) error
}
