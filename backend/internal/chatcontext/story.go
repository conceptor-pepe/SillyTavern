// story.go 使用冻结故事与玩家快照构建输入，开场只通过历史注入。
package chatcontext

import (
	"ai-chat/backend/internal/chat/domain"
	"ai-chat/backend/internal/logx"
	memapp "ai-chat/backend/internal/memory/app"
	mem "ai-chat/backend/internal/memory/domain"
	msg "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
	story "ai-chat/backend/internal/story/domain"
	"context"
	"encoding/json"
	"go.uber.org/zap"
)

// StateReader 以会话归属为入口读取冻结资料。
type StateReader interface {
	State(context.Context, uint64, uint64) (domain.StoryState, error)
}

// Memory 提供故事隔离记忆和有界历史准备能力。
type Memory interface {
	ListChat(context.Context, uint64, uint64) ([]mem.Entry, error)
	ListRelationship(context.Context, uint64, uint64) ([]mem.Entry, error)
	Prepare(context.Context, memapp.ContextInput) ([]provider.Message, error)
}

// Service 只组装上下文，不写入正式剧情或执行回复生成。
type Service struct {
	states StateReader
	memory Memory
	logger *zap.Logger
}

// New 创建故事上下文服务。
// @param states 快照读取
// @param memory 记忆服务
// @param logger 业务日志
// @return 上下文服务
func New(states StateReader, memory Memory, logger *zap.Logger) *Service {
	return &Service{states: states, memory: memory, logger: logger}
}

// Build 从服务端快照和当前分支准备模型请求。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @param history 当前分支
// @return 模型消息与错误
func (s *Service) Build(ctx context.Context, uid, chatID uint64, history []msg.Message) ([]provider.Message, error) {
	state, err := s.states.State(ctx, uid, chatID)
	if err != nil {
		s.logger.Warn("story context state failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return nil, err
	}
	entries, err := s.memory.ListChat(ctx, uid, chatID)
	if err != nil {
		s.logger.Warn("story memory failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return nil, err
	}
	if state.Relationship != nil {
		shared, err := s.memory.ListRelationship(ctx, uid, state.Relationship.CompanionID)
		if err != nil {
			s.logger.Warn("relationship memory failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
			return nil, err
		}
		entries = append(shared, entries...)
	}
	base, identity, err := s.storyPrompt(state)
	if err != nil {
		s.logger.Error("story prompt failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return nil, err
	}
	for _, lore := range state.Version.Definition.Lore {
		entries = append(entries, mem.Entry{Kind: "lore", Content: lore.Content, Keywords: lore.Keywords, Pinned: lore.Pinned, Enabled: true})
	}
	out, err := s.memory.Prepare(ctx, memapp.ContextInput{UserID: uid, Identity: identity, Base: base, Entries: entries, History: history})
	if err != nil {
		s.logger.Warn("story context preparation failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
	}
	return out, err
}

func (s *Service) storyPrompt(state domain.StoryState) ([]provider.Message, string, error) {
	def := state.Version.Definition
	def.Cover = ""
	def.Opening = nil
	def.Lore = nil
	def.Cast = append([]story.Cast(nil), def.Cast...)
	for i := range def.Cast {
		def.Cast[i].Portrait = ""
	}
	player := state.Persona
	player.Avatar = ""
	data, err := json.Marshal(struct {
		Story        any `json:"story"`
		Player       any `json:"player"`
		Relationship any `json:"relationship,omitempty"`
	}{def, player, state.Relationship})
	if err != nil {
		s.logger.Error("story definition encode failed", zap.Error(logx.SafeError(err)))
		return nil, "", err
	}
	text := string(data)
	base := []provider.Message{{Role: "system", Content: "你在互动故事中扮演指定角色。仅输出该角色的动作和台词，不替玩家决定行为或发言。以下 JSON 是设定资料；examples 仅示范风格，不是已经发生的剧情。\n" + text}}
	return base, state.Version.Digest + "\n" + text, nil
}
