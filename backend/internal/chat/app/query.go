// query.go 负责会话查询用例的身份、分页和对象编号校验。
package app

import (
	"context"
	"errors"

	"ai-chat/backend/internal/chat/domain"
)

// 查询错误与 HTTP 状态解耦，由接口层决定对外响应。
var (
	ErrIdentity = errors.New("user identity required")
	ErrQuery    = errors.New("invalid chat query")
)

// Page 保存会话分页结果，不向客户端暴露所有者内部字段。
type Page struct {
	Items []domain.Conversation `json:"items"`
	Total int64                 `json:"total,string"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}

// Query 编排只读会话操作，存储实现由组合层注入。
type Query struct{ repo domain.Repo }

// NewQuery 创建会话查询用例。
func NewQuery(repo domain.Repo) *Query { return &Query{repo: repo} }

// List 限制分页范围后，仅查询当前用户的会话。
func (q *Query) List(ctx context.Context, uid uint64, page, size int) (Page, error) {
	if uid == 0 {
		return Page{}, ErrIdentity
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return Page{}, ErrQuery
	}
	items, total, err := q.repo.List(ctx, uid, page, size)
	if err != nil {
		return Page{}, err
	}
	if items == nil {
		items = []domain.Conversation{}
	}
	return Page{Items: items, Total: total, Page: page, Size: size}, nil
}

// Find 校验用户与对象编号，再按所有者范围查询。
func (q *Query) Find(ctx context.Context, uid, id uint64) (domain.Conversation, error) {
	if uid == 0 {
		return domain.Conversation{}, ErrIdentity
	}
	if id == 0 {
		return domain.Conversation{}, ErrQuery
	}
	return q.repo.Find(ctx, uid, id)
}
