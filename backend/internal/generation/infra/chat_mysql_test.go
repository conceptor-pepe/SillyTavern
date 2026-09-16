// chat_mysql_test.go 验证会话删除与生成写入使用同一真实 MySQL 行锁及事务。
package infra

import (
	"context"
	"errors"
	"testing"
	"time"

	chatdomain "ai-chat/backend/internal/chat/domain"
	chatinfra "ai-chat/backend/internal/chat/infra"
	"ai-chat/backend/internal/generation/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/port"
	"gorm.io/gorm"
)

// TestDeletedChatWrites 已删除或其他用户的会话不能新建任务、保存回复或提交完成事务。
func TestDeletedChatWrites(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "deleted"}[deleted], func(t *testing.T) {
			db := seedDone(t, domain.StatusRunning)
			uid := uint64(8)
			if deleted {
				uid = 7
				if err := chatinfra.NewRepo(db).Delete(t.Context(), uid, 3); err != nil {
					t.Fatal(err)
				}
			}
			gate := chatinfra.NewGate(db)
			tasks := NewRepo(db, gate)
			_, err := tasks.Create(t.Context(), domain.Generation{UserID: uid, ConversationID: 3,
				Provider: "test", Model: "test", Status: domain.StatusPending})
			if !errors.Is(err, chatdomain.ErrNotFound) {
				t.Fatalf("create on invisible chat: %v", err)
			}
			writer := NewDoneWriter(db, gate)
			item := chatReply()
			if _, err := writer.Create(t.Context(), uid, item); !errors.Is(err, chatdomain.ErrNotFound) {
				t.Fatalf("message on invisible chat: %v", err)
			}
			if _, err := writer.SaveDone(t.Context(), uid, 1, item, 100); !errors.Is(err, chatdomain.ErrNotFound) {
				t.Fatalf("completion on invisible chat: %v", err)
			}
			checkDoneCounts(t, db, 0, 0)
			checkChatTask(t, db, domain.StatusRunning, 1)
		})
	}
}

// chatReply 创建带候选的完整回复，回滚时必须同时检查两个消息表。
func chatReply() msgdomain.Message {
	return msgdomain.Message{ConversationID: 3, Role: "assistant", Content: "answer",
		Status: "completed", Variants: []msgdomain.Variant{{VariantNo: 0, Content: "answer"}}}
}

// rollbackGate 在实际写入后主动失败，用于证明生成仓储没有逃逸会话事务。
type rollbackGate struct{ port.ChatGate }

// WithChat 复用真实会话锁并在成功回调后返回固定错误，强制外层事务回滚。
func (g rollbackGate) WithChat(ctx context.Context, uid, id uint64, run func(context.Context) error) error {
	return g.ChatGate.WithChat(ctx, uid, id, func(ctx context.Context) error {
		if err := run(ctx); err != nil {
			return err
		}
		return errors.New("rollback after write")
	})
}

// TestChatWriteRollback 会话事务失败时，新任务、主消息、候选和完成状态必须全部回滚。
func TestChatWriteRollback(t *testing.T) {
	db := seedDone(t, domain.StatusRunning)
	gate := rollbackGate{ChatGate: chatinfra.NewGate(db)}
	task, err := NewRepo(db, gate).Create(t.Context(), domain.Generation{UserID: 7, ConversationID: 3,
		Provider: "test", Model: "test", Status: domain.StatusPending})
	if err == nil || task.ID != 0 {
		t.Fatalf("failed transaction returned task: %+v %v", task, err)
	}
	writer := NewDoneWriter(db, gate)
	item, err := writer.SaveDone(t.Context(), 7, 1, chatReply(), 100)
	if err == nil || item.ID != 0 {
		t.Fatalf("failed transaction returned message: %+v %v", item, err)
	}
	if _, err := writer.Create(t.Context(), 7, chatReply()); err == nil {
		t.Fatal("compatibility write escaped rollback")
	}
	checkDoneCounts(t, db, 0, 0)
	checkChatTask(t, db, domain.StatusRunning, 1)
}

// TestChatDeleteLock 在完成事务持有会话锁时删除必须等待，不能先删除再提交回复。
func TestChatDeleteLock(t *testing.T) {
	db := seedDone(t, domain.StatusRunning)
	gate := chatinfra.NewGate(db)
	held, release := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- gate.WithChat(t.Context(), 7, 3, func(ctx context.Context) error {
			if _, err := NewDoneWriter(db, gate).SaveDone(ctx, 7, 1, chatReply(), 100); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	defer func() {
		close(release)
		if err := <-result; err != nil {
			t.Errorf("completion failed after lock release: %v", err)
		}
	}()
	select {
	case <-held:
	case err := <-result:
		result <- err
		t.Fatalf("completion lock failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("completion did not reach locked state")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	// GORM 可能将回滚错误拼入结果而丢失原错误链，因此同时检查实际截止状态。
	if err := chatinfra.NewRepo(db).Delete(ctx, 7, 3); err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("delete did not wait for completion lock: %v", err)
	}
	var row model.Conversation
	if err := db.First(&row, 3).Error; err != nil || row.DeletedAt != nil {
		t.Fatalf("blocked delete changed chat: %+v %v", row, err)
	}
}

// checkChatTask 确认事务失败没有生成孤立任务，也没有改写原始任务的终态关联。
func checkChatTask(t *testing.T, db *gorm.DB, status string, count int64) {
	t.Helper()
	var tasks []model.Generation
	if err := db.WithContext(t.Context()).Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if int64(len(tasks)) != count || tasks[0].Status != status || tasks[0].MessageID != nil {
		t.Fatalf("task facts changed: %+v", tasks)
	}
}
