// candidates.go 按候选编号组装生成文本，避免交错增量污染主回复。
package app

import (
	"context"
	"errors"
	"io"
	"sort"

	msgdomain "ai-chat/backend/internal/message/domain"
	provider "ai-chat/backend/internal/provider/domain"
)

// readCandidates 校验索引并收集完整候选，回调失败时不返回可落库结果。
func readCandidates(ctx context.Context, stream provider.Stream, count int, send func(provider.Event) error) ([]msgdomain.Variant, error) {
	if count == 0 {
		count = 1
	}
	if count < 1 {
		return nil, errors.New("invalid candidate count")
	}
	texts := make(map[int]string)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		event, err := stream.Next(ctx)
		if errors.Is(err, io.EOF) || (err == nil && event.Type == "done") {
			return collectCandidates(texts, count)
		}
		if err != nil {
			return nil, err
		}
		if event.Type != "delta" || event.Index < 0 || event.Index >= count {
			return nil, errors.New("invalid candidate event")
		}
		texts[event.Index] += event.Text
		if err := sendCandidate(send, event); err != nil {
			return nil, err
		}
	}
}

// collectCandidates 要求请求的每个候选都有响应，并稳定按编号返回。
func collectCandidates(texts map[int]string, count int) ([]msgdomain.Variant, error) {
	if len(texts) != count {
		return nil, errors.New("incomplete candidate response")
	}
	items := make([]msgdomain.Variant, 0, len(texts))
	for index, text := range texts {
		items = append(items, msgdomain.Variant{VariantNo: index, Content: text})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].VariantNo < items[j].VariantNo })
	return items, nil
}

// sendCandidate 保留候选索引，仅将有效文本增量发送给客户端。
func sendCandidate(send func(provider.Event) error, event provider.Event) error {
	if send == nil || event.Text == "" {
		return nil
	}
	return send(event)
}
