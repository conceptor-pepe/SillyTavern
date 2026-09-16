// select_concurrent_test.go 在真实 MySQL 中并发选择同一候选，验证行锁和来源唯一键。
package infra

import (
	"testing"

	"ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
)

// TestConcurrentSelection 验证多个请求得到同一个已提交分支，不重复创建消息。
func TestConcurrentSelection(t *testing.T) {
	db := testdb.Open(t)
	if err := seedSelection(db.WithContext(t.Context())); err != nil {
		t.Fatal(err)
	}
	type result struct {
		item domain.Message
		err  error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	repo := NewRepo(db)
	for range 8 {
		go func() {
			<-start
			item, err := repo.SelectVariant(t.Context(), 7, 8, 21)
			results <- result{item: item, err: err}
		}()
	}
	close(start)
	var selected uint64
	for range 8 {
		value := <-results
		if value.err != nil {
			t.Errorf("select failed: %v", value.err)
			continue
		}
		if selected != 0 && selected != value.item.ID {
			t.Errorf("different selections: %d != %d", selected, value.item.ID)
		}
		selected = value.item.ID
	}
	var count int64
	if err := db.WithContext(t.Context()).Model(&model.Message{}).Where("source_variant_id = ?", 21).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if selected == 0 || count != 1 {
		t.Fatalf("selected=%d persisted=%d", selected, count)
	}
}
