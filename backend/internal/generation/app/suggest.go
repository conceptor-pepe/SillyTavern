// suggest.go 生成不进入剧情历史的玩家回复建议。
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	provider "ai-chat/backend/internal/provider/domain"
	"go.uber.org/zap"
)

const suggestionLimit = 45 * time.Second

// SuggestionInstruction 约束模型只返回三个可直接发送的玩家回复。
const SuggestionInstruction = `请根据以上剧情，以玩家身份给出三条不同语气、可直接发送的下一句回复。只返回 JSON 字符串数组，不要解释，格式：["回复1","回复2","回复3"]。每条不超过 80 个汉字。`

// Suggester 调用模型并校验回复建议结构。
type Suggester struct {
	provider provider.Provider
	logger   *zap.Logger
}

// NewSuggester 创建回复建议生成器。
func NewSuggester(p provider.Provider, logger *zap.Logger) *Suggester {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Suggester{provider: p, logger: logger}
}

// Suggest 返回三条去重后的玩家回复，不创建正式消息。
func (s *Suggester) Suggest(ctx context.Context, model string, messages []provider.Message, maxTokens int) ([]string, error) {
	if s == nil || s.provider == nil || strings.TrimSpace(model) == "" || len(messages) == 0 {
		if s != nil && s.logger != nil {
			s.logger.Warn("suggestion request rejected", zap.String("model", model))
		}
		return nil, errors.New("invalid suggestion request")
	}
	timedCtx, cancel := context.WithTimeout(ctx, suggestionLimit)
	defer cancel()
	input := append([]provider.Message(nil), messages...)
	input = append(input, provider.Message{Role: "system", Content: SuggestionInstruction})
	stream, err := s.provider.Stream(timedCtx, provider.Request{
		Model: model, Messages: input, Stream: true, N: 1, MaxTokens: maxTokens,
	})
	if err != nil {
		s.logger.Error("suggestion stream start failed", zap.String("model", model), zap.Error(err))
		return nil, fmt.Errorf("start suggestion stream: %w", err)
	}
	items, readErr := readCandidates(timedCtx, stream, 1, nil)
	if err := errors.Join(readErr, stream.Close()); err != nil {
		s.logger.Error("suggestion stream read failed", zap.String("model", model), zap.Error(err))
		return nil, fmt.Errorf("read suggestion stream: %w", err)
	}
	return parseSuggestions(items[0].Content, s.logger)
}

// parseSuggestions 接受纯 JSON 或常见的 Markdown JSON 代码块。
func parseSuggestions(value string, logger *zap.Logger) ([]string, error) {
	logger = branchLogger(logger)
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	var items []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &items); err != nil {
		logger.Warn("suggestion JSON rejected", zap.Error(err))
		return nil, errors.New("invalid suggestion response")
	}
	if len(items) != 3 {
		logger.Warn("suggestion count rejected", zap.Int("count", len(items)))
		return nil, errors.New("suggestion response must contain three items")
	}
	seen := make(map[string]struct{}, len(items))
	for index := range items {
		items[index] = strings.TrimSpace(items[index])
		if items[index] == "" || len([]rune(items[index])) > 80 {
			logger.Warn("suggestion content rejected", zap.Int("index", index))
			return nil, errors.New("invalid suggestion content")
		}
		if _, exists := seen[items[index]]; exists {
			logger.Warn("duplicate suggestion rejected", zap.Int("index", index))
			return nil, errors.New("duplicate suggestion content")
		}
		seen[items[index]] = struct{}{}
	}
	return items, nil
}
