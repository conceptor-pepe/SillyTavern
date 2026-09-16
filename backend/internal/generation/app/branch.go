// branch.go 负责生成上下文的父链边界校验，不允许兄弟分支或旧回复混入。
package app

import (
	"context"
	"errors"
	"strings"

	msgdomain "ai-chat/backend/internal/message/domain"
)

// ErrBranch 表示分支损坏或请求内容与已保存的用户消息冲突。
var ErrBranch = errors.New("invalid message branch")

// BranchArgs 标记本次回复所依据的已保存用户消息。
type BranchArgs struct {
	UserID  uint64
	ChatID  uint64
	LeafID  uint64
	Content string
}

// LoadBranch 验证父链完整性后返回历史，叶节点文本不再重复追加。
func LoadBranch(ctx context.Context, repo msgdomain.BranchReader, args BranchArgs) ([]msgdomain.Message, error) {
	if args.UserID == 0 || args.ChatID == 0 || args.LeafID == 0 {
		return nil, ErrBranch
	}
	items, err := repo.Branch(ctx, args.UserID, args.ChatID, args.LeafID)
	if err != nil {
		return nil, err
	}
	if err := checkBranch(items, args); err != nil {
		return nil, err
	}
	return items, nil
}

// checkBranch 检测断链、环和跨会话记录，一百条满窗允许省略更早祖先。
func checkBranch(items []msgdomain.Message, args BranchArgs) error {
	if len(items) == 0 || len(items) > 100 {
		return ErrBranch
	}
	seen := make(map[uint64]bool, len(items))
	for i, item := range items {
		if item.ID == 0 || seen[item.ID] || item.ConversationID != args.ChatID || item.Status != "completed" {
			return ErrBranch
		}
		if i > 0 && (item.ParentID == nil || *item.ParentID != items[i-1].ID) {
			return ErrBranch
		}
		seen[item.ID] = true
	}
	head, leaf := items[0], items[len(items)-1]
	if head.ParentID != nil && (len(items) < 100 || seen[*head.ParentID]) {
		return ErrBranch
	}
	if leaf.ID != args.LeafID || leaf.Role != "user" || strings.TrimSpace(leaf.Content) == "" {
		return ErrBranch
	}
	if args.Content != "" && args.Content != leaf.Content {
		return ErrBranch
	}
	return nil
}
