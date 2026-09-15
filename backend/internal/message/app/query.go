// query.go 负责消息历史查询的身份、会话归属和分页校验。
package app

import (
	"context"
	"errors"

	"ai-chat/backend/internal/message/domain"
)

// ErrQuery 表示消息请求不满足基本边界。
var ErrQuery = errors.New("invalid message query")

// Query 保存消息查询用例依赖。
type Query struct {
	messages domain.Repo
	chats    domain.ChatRepo
}

// MessageRepo 标记消息查询用例所需的消息存储能力。
type MessageRepo = domain.Repo

// ChatRepo 标记消息查询用例所需的会话存储能力。
type ChatRepo = domain.ChatRepo

// NewQuery 创建消息查询用例。
func NewQuery(messages domain.Repo, chats domain.ChatRepo) *Query {
	return &Query{messages: messages, chats: chats}
}

// List 先验证会话归属，再查询消息历史。
func (q *Query) List(ctx context.Context, uid, chatID uint64, page, size int) ([]domain.Message, int64, error) {
	if uid == 0 || chatID == 0 || page < 1 || size < 1 || size > 100 {
		return nil, 0, ErrQuery
	}
	owned, err := q.chats.Owns(ctx, uid, chatID)
	if err != nil {
		return nil, 0, err
	}
	if !owned {
		return nil, 0, ErrQuery
	}
	return q.messages.List(ctx, uid, chatID, page, size)
}
