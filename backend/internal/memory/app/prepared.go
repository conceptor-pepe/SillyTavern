// prepared.go 故事上下文复用记忆预算及摘要能力，不读取可变角色资料。
package app

import (
	char "ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/memory/domain"
	msg "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"context"
	"go.uber.org/zap"
)

// ContextInput 的 Identity 必须覆盖冻结版本与玩家身份。
type ContextInput struct {
	UserID   uint64
	Identity string
	Base     []provider.Message
	Entries  []domain.Entry
	History  []msg.Message
}

// Prepare 使用调用方已授权的设定和记忆，避免故事误读角色记忆。
// @param ctx 请求上下文
// @param in 上下文资料
// @return 模型消息与错误
func (s *Service) Prepare(ctx context.Context, in ContextInput) ([]provider.Message, error) {
	if pinnedSize(in.Entries, domain.Entry{}) > 6000 {
		s.logger.Warn("story pinned memory exceeds budget", zap.Uint64("user_id", in.UserID))
		return nil, domain.ErrBudget
	}
	identity := char.Character{UserID: in.UserID, Description: in.Identity}
	base := append([]provider.Message(nil), in.Base...)
	return s.prepare(ctx, identity, base, in.Entries, in.History)
}
