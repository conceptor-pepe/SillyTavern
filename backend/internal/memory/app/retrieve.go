// retrieve.go 混合关键词与可选语义向量，世界书依赖明确触发词以避免无关设定污染。
package app

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/memory/domain"
	"context"
	"go.uber.org/zap"
	"math"
	"sort"
	"strings"
	"time"
)

type scoredEntry struct {
	item  domain.Entry
	score float64
}

// recall 保留固定事实优先级，召回内容总量受独立预算限制。
func (s *Service) recall(ctx context.Context, items []domain.Entry, query string) string {
	vector := s.queryVector(ctx, query, items)
	ranked := make([]scoredEntry, 0, len(items))
	for _, item := range items {
		score := relevance(item, query, vector, s.embed)
		if item.Enabled && score > 0 {
			ranked = append(ranked, scoredEntry{item, score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	var out strings.Builder
	for _, entry := range ranked {
		if out.Len()+len(entry.item.Content)+3 > 6000 {
			continue
		}
		out.WriteString("\n- " + entry.item.Content)
	}
	return out.String()
}

// queryVector 没有同模型索引时不发出无意义的外部请求。
func (s *Service) queryVector(ctx context.Context, query string, items []domain.Entry) []float64 {
	if s.embed == nil || query == "" {
		return nil
	}
	indexed := false
	for _, item := range items {
		indexed = indexed || (item.Enabled && item.VectorModel == s.embed.Model() && len(item.Vector) > 0)
	}
	if !indexed {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := s.embed.Embed(ctx, []string{query})
	if err != nil {
		s.logger.Warn("memory recall using keywords", zap.Error(logx.SafeError(err)))
		return nil
	}
	if len(rows) != 1 {
		return nil
	}
	return rows[0]
}

// relevance 只对事实启用语义补充；世界书按用户配置的触发规则执行。
func relevance(item domain.Entry, query string, vector []float64, embed domain.Embedder) float64 {
	if item.Pinned {
		return 100
	}
	query = strings.ToLower(query)
	for _, key := range item.Keywords {
		if strings.Contains(query, strings.ToLower(key)) {
			return 20
		}
	}
	if item.Kind == "lore" {
		return 0
	}
	score := overlap(item.Content, query)
	if embed != nil && item.VectorModel == embed.Model() {
		score = math.Max(score, cosine(item.Vector, vector))
	}
	if score < 0.15 {
		return 0
	}
	return score
}

// overlap 中文采用相邻双字，英文也可获得不依赖分词服务的基础检索。
func overlap(content, query string) float64 {
	runes := []rune(strings.ToLower(content))
	hits, total := 0, 0
	for i := 1; i < len(runes); i++ {
		pair := string(runes[i-1 : i+1])
		if strings.TrimSpace(pair) == "" {
			continue
		}
		total++
		if strings.Contains(query, pair) {
			hits++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// cosine 向量维度或数值异常时拒绝比较，防止不同模型混用。
func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, aa, bb float64
	for i, v := range a {
		dot += v * b[i]
		aa += v * v
		bb += b[i] * b[i]
	}
	if aa == 0 || bb == 0 {
		return 0
	}
	score := dot / math.Sqrt(aa*bb)
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0
	}
	return score
}
