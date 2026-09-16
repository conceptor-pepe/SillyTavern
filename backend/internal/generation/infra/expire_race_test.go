// expire_race_test.go 验证真实 MySQL 中清理与完成、取消竞争时只有一个终态生效。
package infra

import (
	"context"
	"testing"
	"time"

	chatinfra "ai-chat/backend/internal/chat/infra"
	"ai-chat/backend/internal/generation/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
)

// expiryResult 保存数据库更新结果，协程通过通道传递而非共享可变状态。
type expiryResult struct {
	count int64
	err   error
}

// TestExpireRace 用真实并发数据库请求检验终态保护，不以 Go race 检查代替数据库竞争。
func TestExpireRace(t *testing.T) {
	for _, action := range []string{"complete", "cancel"} {
		t.Run(action, func(t *testing.T) {
			for attempt := 0; attempt < 4; attempt++ {
				db := seedDone(t, domain.StatusRunning)
				gate := chatinfra.NewGate(db)
				repo := NewRepo(db, gate)
				result, actionErr := raceExpiry(t.Context(), repo, NewDoneWriter(db, gate), action)
				if result.err != nil {
					t.Fatal(result.err)
				}
				task, err := repo.Find(t.Context(), 7, 1)
				if err != nil {
					t.Fatal(err)
				}
				checkRaceTask(t, task, result, actionErr, action)
				count := int64(0)
				if action == "complete" && actionErr == nil {
					count = 1
				}
				checkDoneCounts(t, db, count, count)
			}
		})
	}
}

// raceExpiry 同时释放两个数据库操作，等待全部退出后才返回结果。
func raceExpiry(ctx context.Context, repo *Repo, writer *DoneWriter, action string) (expiryResult, error) {
	start := make(chan struct{})
	expired := make(chan expiryResult, 1)
	acted := make(chan error, 1)
	go func() {
		<-start
		count, err := repo.Expire(ctx, time.Now().Add(time.Hour).Unix())
		expired <- expiryResult{count: count, err: err}
	}()
	go func() {
		<-start
		acted <- raceAction(ctx, repo, writer, action)
	}()
	close(start)
	return <-expired, <-acted
}

// raceAction 复用生产完成事务或取消条件更新，不以直接修改表绕过状态保护。
func raceAction(ctx context.Context, repo *Repo, writer *DoneWriter, action string) error {
	if action == "cancel" {
		finished := time.Now().Unix()
		return repo.Move(ctx, 7, 1, []string{domain.StatusPending, domain.StatusRunning},
			domain.Generation{Status: domain.StatusCancelled, FinishedAt: &finished})
	}
	item := msgdomain.Message{ConversationID: 3, Role: "assistant", Content: "answer",
		Status: "completed", Variants: []msgdomain.Variant{{VariantNo: 0, Content: "answer"}}}
	_, err := writer.SaveDone(ctx, 7, 1, item, time.Now().Unix())
	return err
}

// checkRaceTask 核对胜出操作和数据库事实一致，禁止两个调用同时宣称成功。
func checkRaceTask(t *testing.T, task domain.Generation, expiry expiryResult, actionErr error, action string) {
	t.Helper()
	if task.FinishedAt == nil {
		t.Fatalf("terminal state has no finish time: %+v", task)
	}
	if expiry.count == 1 {
		if actionErr == nil || task.Status != domain.StatusFailed || task.ErrorCode != "generation_timeout" {
			t.Fatalf("expiry winner overwritten: task=%+v actionErr=%v", task, actionErr)
		}
		return
	}
	want := domain.StatusCompleted
	if action == "cancel" {
		want = domain.StatusCancelled
	}
	if expiry.count != 0 || actionErr != nil || task.Status != want || task.ErrorCode != "" {
		t.Fatalf("action winner inconsistent: task=%+v expiry=%+v actionErr=%v", task, expiry, actionErr)
	}
}
