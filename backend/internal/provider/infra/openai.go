// openai.go 实现 OpenAI 兼容接口的流式 Provider。
package infra

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	provider "ai-chat/backend/internal/provider/domain"
)

// OpenAI 保存兼容 Chat Completions 接口的 HTTP 配置。
type OpenAI struct {
	URL    string
	Key    string
	Client *http.Client
}

// Stream 创建一个可取消的模型事件流。
func (p *OpenAI) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+p.Key)
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= http.StatusBadRequest {
		closeErr := response.Body.Close()
		if closeErr != nil {
			return nil, errors.Join(errors.New("provider request failed"), closeErr)
		}
		return nil, errors.New("provider request failed")
	}
	stop := make(chan struct{})
	go closeOnCancel(ctx, response.Body, stop)
	return &stream{body: response.Body, scan: bufio.NewScanner(response.Body), stop: stop}, nil
}

type stream struct {
	body    io.ReadCloser
	scan    *bufio.Scanner
	stop    chan struct{}
	once    sync.Once
	pending []provider.Event
}

// Next 读取一条 SSE data 事件并转换为统一事件。
func (s *stream) Next(ctx context.Context) (provider.Event, error) {
	if err := ctx.Err(); err != nil {
		return provider.Event{}, err
	}
	if len(s.pending) > 0 {
		return s.popEvent(), nil
	}
	for s.scan.Scan() {
		select {
		case <-ctx.Done():
			return provider.Event{}, ctx.Err()
		default:
		}
		line := strings.TrimSpace(s.scan.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if value == "[DONE]" {
			return provider.Event{Type: "done"}, nil
		}
		events, err := parseEvents(value)
		if err != nil {
			return provider.Event{}, err
		}
		s.pending = events
		return s.popEvent(), nil
	}
	if err := s.scan.Err(); err != nil {
		return provider.Event{}, err
	}
	return provider.Event{Type: "done"}, io.EOF
}

// popEvent 先返回同一数据帧内的剩余候选，避免读取下一帧时丢失内容。
func (s *stream) popEvent() provider.Event {
	event := s.pending[0]
	s.pending = s.pending[1:]
	return event
}

// Close 释放 Provider 的响应连接。
func (s *stream) Close() error {
	s.once.Do(func() { close(s.stop) })
	return s.body.Close()
}

// closeOnCancel 在请求取消时关闭响应体，打断底层阻塞读取。
func closeOnCancel(ctx context.Context, body io.ReadCloser, stop <-chan struct{}) {
	select {
	case <-ctx.Done():
		// 取消路径只为打断读取，关闭错误不能替代原始取消原因。
		_ = body.Close()
	case <-stop:
	}
}

// parseEvents 保留数据帧中的全部候选及顺序，不暴露供应商原始响应。
func parseEvents(value string) ([]provider.Event, error) {
	var data struct {
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return nil, err
	}
	if len(data.Choices) == 0 {
		return []provider.Event{{Type: "delta"}}, nil
	}
	events := make([]provider.Event, 0, len(data.Choices))
	for _, choice := range data.Choices {
		events = append(events, provider.Event{Type: "delta", Index: choice.Index, Text: choice.Delta.Content})
	}
	return events, nil
}
