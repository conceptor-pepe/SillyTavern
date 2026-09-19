// service.go 编排记忆维护与模型上下文构建，模型失败不能写入半成品记忆。
package app

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/memory/domain"
	msg "ai-chat/backend/internal/message/domain"
	"context"
	"go.uber.org/zap"
	"strings"
	"time"
)

// Service 保存持久化与模型能力，不保存请求 Context。
type Service struct {
	repo    domain.Repo
	history msg.BranchReader
	summary domain.Summarizer
	embed   domain.Embedder
	extract domain.CandidateExtractor
	logger  *zap.Logger
	budget  int
}

// Options 将可选模型能力与预算集中配置，避免构造函数位置参数混淆。
type Options struct {
	Summary domain.Summarizer
	Embed   domain.Embedder
	Extract domain.CandidateExtractor
	Budget  int
}

// New 创建记忆服务，预算采用保守 UTF-8 字节估算。
func New(repo domain.Repo, history msg.BranchReader, logger *zap.Logger, options Options) *Service {
	if options.Budget <= 0 {
		options.Budget = 24000
	}
	return &Service{repo: repo, history: history, summary: options.Summary, embed: options.Embed,
		extract: options.Extract, logger: logger, budget: options.Budget}
}

// List 返回当前角色可编辑的长期记忆和世界书。
func (s *Service) List(ctx context.Context, uid, charID uint64) ([]domain.Entry, error) {
	items, err := s.repo.List(ctx, uid, charID)
	if err != nil {
		s.logger.Warn("memory list failed", zap.Error(logx.SafeError(err)))
	}
	return items, err
}

// Save 校验内容并在可用时建立向量，供应商失败降级为关键词检索。
func (s *Service) Save(ctx context.Context, item domain.Entry) (domain.Entry, error) {
	if err := validate(&item); err != nil {
		s.logger.Warn("memory input rejected", zap.Error(err))
		return item, err
	}
	entries, err := s.scopeEntries(ctx, item)
	if err != nil {
		s.logger.Warn("memory scope rejected", zap.Error(logx.SafeError(err)))
		return item, err
	}
	if pinnedSize(entries, item) > 6000 {
		s.logger.Warn("pinned memory budget rejected", zap.Uint64("user_id", item.UserID))
		return item, domain.ErrInvalid
	}
	item.Vector, item.VectorModel = nil, ""
	if s.embed != nil {
		s.index(ctx, &item)
	}
	out, err := s.repo.Save(ctx, item)
	if err != nil {
		s.logger.Error("memory save failed", zap.Error(logx.SafeError(err)))
		return out, err
	}
	s.logger.Info("memory saved", zap.Uint64("user_id", item.UserID), zap.Uint64("character_id", item.CharacterID), zap.Uint64("memory_id", out.ID))
	return out, nil
}

// Delete 用户主动遗忘立即删除正文与索引。
func (s *Service) Delete(ctx context.Context, uid, charID, id uint64) error {
	if err := s.repo.Delete(ctx, uid, charID, id); err != nil {
		s.logger.Warn("memory delete failed", zap.Error(logx.SafeError(err)))
		return err
	}
	s.logger.Info("memory deleted", zap.Uint64("user_id", uid), zap.Uint64("memory_id", id))
	return nil
}

// index 为单次向量调用设定独立时限，原始错误不写入日志。
func (s *Service) index(ctx context.Context, item *domain.Entry) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	vectors, err := s.embed.Embed(ctx, []string{item.Content})
	if err != nil {
		s.logger.Warn("memory embedding unavailable", zap.Error(logx.SafeError(err)))
		return
	}
	if len(vectors) == 1 && len(vectors[0]) > 0 {
		item.Vector, item.VectorModel = vectors[0], s.embed.Model()
	}
}

// validate 限制召回输入大小，世界书必须有触发词或明确常驻。
func validate(item *domain.Entry) error {
	item.Content = strings.TrimSpace(item.Content)
	if item.UserID == 0 || (item.CharacterID == 0 && item.ChatID == 0) || (item.CharacterID != 0 && item.ChatID != 0) || item.Content == "" || len(item.Content) > 4000 {
		return domain.ErrInvalid
	}
	if item.Kind != "fact" && item.Kind != "lore" {
		return domain.ErrInvalid
	}
	if len(item.Keywords) > 16 {
		return domain.ErrInvalid
	}
	for i, key := range item.Keywords {
		key = strings.TrimSpace(key)
		if key == "" || len([]rune(key)) > 64 {
			return domain.ErrInvalid
		}
		item.Keywords[i] = key
	}
	if item.Kind == "lore" && !item.Pinned && len(item.Keywords) == 0 {
		return domain.ErrInvalid
	}
	return nil
}

// pinnedSize 常驻记忆有独立配额，不能在召回时静默丢掉用户要求始终记住的内容。
func pinnedSize(items []domain.Entry, current domain.Entry) int {
	size := 0
	for _, item := range items {
		if item.ID != current.ID && item.Enabled && item.Pinned {
			size += len(item.Content) + 3
		}
	}
	if current.Enabled && current.Pinned {
		size += len(current.Content) + 3
	}
	return size
}
