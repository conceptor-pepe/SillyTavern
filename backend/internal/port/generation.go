// generation.go 定义生成域向会话删除流程提供的取消能力。
package port

import "context"

// ChatTasks 在调用方事务内取消会话的未结束任务，不改写已有终态。
type ChatTasks interface {
	CancelChat(ctx context.Context, userID, chatID uint64) error
}
