// context_test.go 验证分支摘要、预算与记忆失效规则，不调用真实模型。
package app

import (
	char "ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/memory/domain"
	msg "ai-chat/backend/internal/message/domain"
	"context"
	"errors"
	"go.uber.org/zap"
	"strings"
	"testing"
)

type memoryFake struct {
	entries   []domain.Entry
	snapshots []domain.Summary
}

// List 返回测试记忆列表。
func (f *memoryFake) List(context.Context, uint64, uint64) ([]domain.Entry, error) {
	return f.entries, nil
}

// Save 模拟记忆保存。
func (f *memoryFake) Save(_ context.Context, item domain.Entry) (domain.Entry, error) {
	return item, nil
}

// Delete 模拟记忆遗忘。
func (f *memoryFake) Delete(context.Context, uint64, uint64, uint64) error { return nil }

// Summaries 返回已保存摘要候选。
func (f *memoryFake) Summaries(context.Context, uint64, uint64) ([]domain.Summary, error) {
	return f.snapshots, nil
}

// SaveSummary 记录摘要用于缓存断言。
func (f *memoryFake) SaveSummary(_ context.Context, item domain.Summary) error {
	f.snapshots = append(f.snapshots, item)
	return nil
}

type summaryFake struct {
	calls int
	err   error
	input string
}

// Summarize 记录摘要调用并返回受控结果。
func (f *summaryFake) Summarize(_ context.Context, previous, text string) (string, error) {
	f.calls++
	f.input = text
	return "用户约定周五看电影。", f.err
}

type branchFake struct{ rows []msg.Message }

// Branch 按祖先窗口返回测试消息。
func (f branchFake) Branch(_ context.Context, uid, chat, id uint64) ([]msg.Message, error) {
	for i, item := range f.rows {
		if item.ID != id {
			continue
		}
		start := max(0, i-99)
		return f.rows[start : i+1], nil
	}
	return nil, errors.New("missing")
}
func historyFixture(n int) []msg.Message {
	out := make([]msg.Message, n)
	for i := range out {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		out[i] = msg.Message{ID: uint64(i + 1), ConversationID: 9, Role: role, Content: strings.Repeat("对话", 60), Status: "completed"}
		if i > 0 {
			parent := uint64(i)
			out[i].ParentID = &parent
		}
	}
	return out
}

// TestContextSummary 验证超出近期窗口仍能摘要旧约定，再次请求复用缓存。
func TestContextSummary(t *testing.T) {
	history := historyFixture(121)
	history[0].Content = "记住：周五一起看电影"
	repo := &memoryFake{entries: []domain.Entry{{Kind: "fact", Content: "用户喜欢叫小鹿", Pinned: true, Enabled: true}}}
	summary := &summaryFake{}
	svc := New(repo, branchFake{history}, zap.NewNop(), Options{Summary: summary, Budget: 7000})
	character := char.Character{ID: 1, UserID: 2, Name: "小满"}
	prompt, err := svc.Build(t.Context(), character, history[21:])
	if err != nil {
		t.Fatal(err)
	}
	if promptCost(prompt) > 7000 || prompt[len(prompt)-1].Content != history[120].Content {
		t.Fatal("budget/current question lost")
	}
	if !strings.Contains(prompt[1].Content, "小鹿") || summary.calls == 0 {
		t.Fatal("memory or summary missing")
	}
	calls := summary.calls
	if _, err := svc.Build(t.Context(), character, history[21:]); err != nil {
		t.Fatal(err)
	}
	if summary.calls != calls {
		t.Fatal("unchanged history recomputed")
	}
	history[0].Content = "改为周六看电影"
	if _, err := svc.Build(t.Context(), character, history[21:]); err != nil {
		t.Fatal(err)
	}
	if summary.calls == calls {
		t.Fatal("edited source reused stale summary")
	}
}

// TestSummaryBranchIsolation 同父候选、角色变化及删除来源不得复用旧摘要。
func TestSummaryBranchIsolation(t *testing.T) {
	items := historyFixture(4)
	character := char.Character{Name: "A"}
	snapshot := domain.Summary{EndID: 4, Digest: historyDigest(character, items), Content: "old"}
	if _, end := cachedSummary(character, items, []domain.Summary{snapshot}); end != 4 {
		t.Fatal("valid prefix rejected")
	}
	items[1].Content = "另一条剧情"
	if _, end := cachedSummary(character, items, []domain.Summary{snapshot}); end != 0 {
		t.Fatal("branch pollution")
	}
	if _, end := cachedSummary(character, items[:3], []domain.Summary{snapshot}); end != 0 {
		t.Fatal("deleted source used")
	}
}

// TestContextFailures 摘要故障和超长当前问题必须明确失败，不能丢掉问题后继续回答。
func TestContextFailures(t *testing.T) {
	rows := historyFixture(20)
	summary := &summaryFake{err: errors.New("offline")}
	repo := &memoryFake{}
	svc := New(repo, branchFake{rows}, zap.NewNop(), Options{Summary: summary, Budget: 6000})
	if _, err := svc.Build(t.Context(), char.Character{UserID: 1, ID: 1}, rows); err == nil {
		t.Fatal("summary failure ignored")
	}
	rows = historyFixture(1)
	rows[0].Content = strings.Repeat("x", 7000)
	if _, err := svc.Build(t.Context(), char.Character{UserID: 1, ID: 1}, rows); !errors.Is(err, domain.ErrBudget) {
		t.Fatalf("want budget error, got %v", err)
	}
}

// TestWorldInfoRetrieval 世界书按关键词或常驻触发，disabled 永不注入。
func TestWorldInfoRetrieval(t *testing.T) {
	svc := New(&memoryFake{}, nil, zap.NewNop(), Options{})
	items := []domain.Entry{{Kind: "lore", Content: "月港在北方", Keywords: []string{"月港"}, Enabled: true}, {Kind: "lore", Content: "隐藏设定", Keywords: []string{"月港"}, Enabled: false}, {Kind: "fact", Content: "固定称呼小鹿", Pinned: true, Enabled: true}}
	got := svc.recall(t.Context(), items, "去月港")
	if !strings.Contains(got, "北方") || !strings.Contains(got, "小鹿") || strings.Contains(got, "隐藏") {
		t.Fatal(got)
	}
	if strings.Contains(svc.recall(t.Context(), items, "你好"), "北方") {
		t.Fatal("untriggered lore included")
	}
	if cosine([]float64{1, 0}, []float64{1, 0}) != 1 || cosine([]float64{1}, []float64{1, 0}) != 0 {
		t.Fatal("vector mismatch")
	}
}

// TestPinnedBudget 常驻记忆总量超过独立预算时拒绝保存，不能静默漏掉常驻事实。
func TestPinnedBudget(t *testing.T) {
	repo := &memoryFake{entries: []domain.Entry{{ID: 1, Content: strings.Repeat("x", 4000), Pinned: true, Enabled: true}}}
	svc := New(repo, nil, zap.NewNop(), Options{})
	_, err := svc.Save(t.Context(), domain.Entry{UserID: 1, CharacterID: 2, Kind: "fact", Content: strings.Repeat("x", 3000), Pinned: true, Enabled: true})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("pinned budget ignored")
	}
}
