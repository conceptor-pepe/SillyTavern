// summary.go 增量压缩历史前缀，并验证内容、父链和角色设定的散列。
package app

import (
	char "ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/memory/domain"
	msg "ai-chat/backend/internal/message/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go.uber.org/zap"
	"strconv"
	"strings"
)

// compress 只复用当前祖先链上且未被编辑的摘要，不跨会话注入剧情。
func (s *Service) compress(ctx context.Context, character char.Character, items []msg.Message) (string, error) {
	saved, err := s.repo.Summaries(ctx, character.UserID, items[0].ConversationID)
	if err != nil {
		s.logger.Warn("summary read failed", zap.Error(logx.SafeError(err)))
		return "", err
	}
	text, start := cachedSummary(character, items, saved)
	if start == len(items) {
		return text, nil
	}
	if s.summary == nil {
		return "", errors.New("summary provider unavailable")
	}
	for start < len(items) {
		end := chunkEnd(items, start)
		text, err = s.summary.Summarize(ctx, text, transcriptText(items[start:end]))
		if err != nil {
			s.logger.Warn("summary generation failed", zap.Error(logx.SafeError(err)))
			return "", err
		}
		if strings.TrimSpace(text) == "" || len(text) > 4000 {
			return "", errors.New("invalid summary output")
		}
		snapshot := domain.Summary{UserID: character.UserID, ChatID: items[0].ConversationID, EndID: items[end-1].ID, Digest: historyDigest(character, items[:end]), Content: text}
		if err := s.repo.SaveSummary(ctx, snapshot); err != nil {
			s.logger.Warn("summary save failed", zap.Error(logx.SafeError(err)))
			return "", err
		}
		s.logger.Info("summary saved", zap.Uint64("user_id", character.UserID), zap.Uint64("chat_id", snapshot.ChatID), zap.Uint64("end_id", snapshot.EndID))
		start = end
	}
	return text, nil
}

// cachedSummary 返回覆盖最长有效前缀的摘要，编辑或删除来源后旧摘要自动失效。
func cachedSummary(character char.Character, items []msg.Message, saved []domain.Summary) (string, int) {
	positions := make(map[uint64]int, len(items))
	for i, item := range items {
		positions[item.ID] = i + 1
	}
	text, end := "", 0
	for _, snapshot := range saved {
		size := positions[snapshot.EndID]
		if size <= end || snapshot.Digest != historyDigest(character, items[:size]) {
			continue
		}
		text, end = snapshot.Content, size
	}
	return text, end
}

// chunkEnd 控制每次摘要输入，单条超长文本由供应商适配器明确拒绝。
func chunkEnd(items []msg.Message, start int) int {
	size, end := 0, start
	for end < len(items) && (end == start || size+len(items[end].Content) < 12000) {
		size += len(items[end].Content) + 32
		end++
	}
	return end
}

// historyDigest 只编码可序列化的身份与原文，不依赖扩展字段是否为合法 JSON。
func historyDigest(character char.Character, items []msg.Message) string {
	type source struct {
		ID            uint64
		Parent        *uint64
		Role, Content string
	}
	rows := make([]source, 0, len(items))
	for _, item := range items {
		rows = append(rows, source{item.ID, item.ParentID, item.Role, item.Content})
	}
	// 此结构只含原始值类型，JSON 编码不会失败。
	type personaSource struct{ Name, Description, Personality, Scenario, FirstMessage, Gender, Age, MessageSample string }
	persona := personaSource{character.Name, character.Description, character.Personality, character.Scenario, character.FirstMessage, character.Gender, character.Age, character.MessageSample}
	data, _ := json.Marshal(struct {
		Character personaSource
		Messages  []source
	}{persona, rows})
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// transcriptText 保留说话人与来源编号，避免把角色虚构当成用户陈述。
func transcriptText(items []msg.Message) string {
	var out strings.Builder
	for _, item := range items {
		out.WriteString("[" + item.Role + " #" + strconv.FormatUint(item.ID, 10) + "] " + item.Content + "\n")
	}
	return out.String()
}
