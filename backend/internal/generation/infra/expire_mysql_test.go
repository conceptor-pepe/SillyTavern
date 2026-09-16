// expire_mysql_test.go 验证真实 MySQL 超时补偿的状态、时间边界与迟到结果回滚。
package infra

import (
	"errors"
	"testing"
	"time"

	chatinfra "ai-chat/backend/internal/chat/infra"
	"ai-chat/backend/internal/generation/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
)

// expiryCase 描述一个状态和时间组合，避免各场景使用不同判断规则。
type expiryCase struct {
	name    string
	status  string
	created time.Time
	started *int64
	expired bool
}

// expiryCases 覆盖排队、缺失启动时间、严格截止边界和已有终态。
func expiryCases(cutoff time.Time) []expiryCase {
	old, fresh := cutoff.Add(-time.Second), cutoff.Add(time.Second)
	past, edge, future := old.Unix(), cutoff.Unix(), fresh.Unix()
	return []expiryCase{
		{"pending old", domain.StatusPending, old, nil, true},
		{"pending edge", domain.StatusPending, cutoff, nil, false},
		{"pending fresh", domain.StatusPending, fresh, nil, false},
		{"pending stray start", domain.StatusPending, old, &future, true},
		{"running old", domain.StatusRunning, old, &past, true},
		{"running edge", domain.StatusRunning, old, &edge, false},
		{"running fresh", domain.StatusRunning, old, &future, false},
		{"running missing old", domain.StatusRunning, old, nil, true},
		{"running missing edge", domain.StatusRunning, cutoff, nil, false},
		{"running missing fresh", domain.StatusRunning, fresh, nil, false},
		{"completed", domain.StatusCompleted, old, &past, false},
		{"failed", domain.StatusFailed, old, &past, false},
		{"cancelled", domain.StatusCancelled, old, &past, false},
	}
}

// TestExpireMySQL 验证超时匹配、重复清理幂等及不可变字段保持。
func TestExpireMySQL(t *testing.T) {
	db := testdb.Open(t).WithContext(t.Context())
	if err := db.AutoMigrate(&model.Generation{}); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	for _, scenario := range expiryCases(cutoff) {
		t.Run(scenario.name, func(t *testing.T) {
			row := model.Generation{Base: model.Base{CreatedAt: scenario.created},
				UserID: 7, ConversationID: 3, Provider: "test", Model: "test",
				Status: scenario.status, StartedAt: scenario.started, ErrorCode: "original"}
			if err := db.WithContext(t.Context()).Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			repo := NewRepo(db, chatinfra.NewGate(db))
			count, err := repo.Expire(t.Context(), cutoff.Unix())
			want := int64(0)
			if scenario.expired {
				want = 1
			}
			if err != nil || count != want {
				t.Fatalf("count=%d want=%d err=%v", count, want, err)
			}
			var got model.Generation
			if err := db.WithContext(t.Context()).First(&got, row.ID).Error; err != nil {
				t.Fatal(err)
			}
			checkExpired(t, row, got, scenario.expired)
			if count, err := repo.Expire(t.Context(), cutoff.Unix()); err != nil || count != 0 {
				t.Fatalf("repeated cleanup: count=%d err=%v", count, err)
			}
		})
	}
}

// checkExpired 校验终态和归属字段，避免仅检查受影响行数遗漏数据破坏。
func checkExpired(t *testing.T, before, after model.Generation, expired bool) {
	t.Helper()
	if before.UserID != after.UserID || before.ConversationID != after.ConversationID ||
		!before.CreatedAt.Equal(after.CreatedAt) || !sameTime(before.StartedAt, after.StartedAt) {
		t.Fatalf("ownership or source time changed: before=%+v after=%+v", before, after)
	}
	if !expired {
		if after.Status != before.Status || after.ErrorCode != before.ErrorCode ||
			!sameTime(before.FinishedAt, after.FinishedAt) || !before.UpdatedAt.Equal(after.UpdatedAt) {
			t.Fatalf("unexpired task changed: before=%+v after=%+v", before, after)
		}
		return
	}
	if after.Status != domain.StatusFailed || after.ErrorCode != "generation_timeout" ||
		after.ErrorMessage != "generation timed out" || after.FinishedAt == nil {
		t.Fatalf("incomplete expiry: %+v", after)
	}
}

// sameTime 比较可空秒级时间，不把缺失时间与零值混为一谈。
func sameTime(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// TestExpireLateDone 验证超时后迟到的消息及候选回滚，任务不能重新启动。
func TestExpireLateDone(t *testing.T) {
	db := seedDone(t, domain.StatusRunning)
	repo := NewRepo(db, chatinfra.NewGate(db))
	if count, err := repo.Expire(t.Context(), time.Now().Add(time.Hour).Unix()); err != nil || count != 1 {
		t.Fatalf("expiry: count=%d err=%v", count, err)
	}
	item := msgdomain.Message{ConversationID: 3, Role: "assistant", Content: "late",
		Status: "completed", Variants: []msgdomain.Variant{{VariantNo: 0, Content: "late"}}}
	if _, err := NewDoneWriter(db, chatinfra.NewGate(db)).SaveDone(t.Context(), 7, 1, item, time.Now().Unix()); err == nil {
		t.Fatal("expired task accepted late completion")
	}
	checkDoneCounts(t, db, 0, 0)
	err := repo.Move(t.Context(), 7, 1, []string{domain.StatusPending},
		domain.Generation{Status: domain.StatusRunning})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired task restarted: %v", err)
	}
	task, err := repo.Find(t.Context(), 7, 1)
	if err != nil || task.Status != domain.StatusFailed || task.ErrorCode != "generation_timeout" {
		t.Fatalf("expiry overwritten: task=%+v err=%v", task, err)
	}
}
