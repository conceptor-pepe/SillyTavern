// remove_race_test.go 验证删除与收藏、创建、启动和完成竞争时没有孤立活动数据。
package infra

import (
	"errors"
	"testing"

	chatapp "ai-chat/backend/internal/chat/app"
	chatdomain "ai-chat/backend/internal/chat/domain"
	chatinfra "ai-chat/backend/internal/chat/infra"
	genapp "ai-chat/backend/internal/generation/app"
	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// TestRemoveRace 双方同时争抢真实会话行锁，允许任一合法串行顺序。
func TestRemoveRace(t *testing.T) {
	for _, action := range []string{"favorite", "create", "start", "complete"} {
		t.Run(action, func(t *testing.T) {
			db := seedDone(t, "pending")
			if action == "complete" {
				if err := db.Model(&model.Generation{}).Where("id = ?", 1).Update("status", "running").Error; err != nil {
					t.Fatal(err)
				}
			}
			gate := chatinfra.NewGate(db)
			tasks := genapp.New(NewRepo(db, gate))
			remove := chatapp.NewRemover(chatinfra.NewRepo(db), gate, tasks)
			start, result := make(chan struct{}), make(chan error, 1)
			go func() {
				<-start
				result <- competeRemove(t, db, action)
			}()
			close(start)
			err := remove.Delete(t.Context(), 7, 3)
			other := <-result
			if err != nil {
				t.Fatal(err)
			}
			if other != nil && !errors.Is(other, domain.ErrNotFound) && !errors.Is(other, chatdomain.ErrNotFound) {
				t.Fatalf("unexpected competitor error: %v", other)
			}
			checkOrphans(t, db)
		})
	}
}

// competeRemove 使用生产仓储执行争抢动作，不伪造事务或锁。
func competeRemove(t *testing.T, db *gorm.DB, action string) error {
	gate := chatinfra.NewGate(db)
	tasks := genapp.New(NewRepo(db, gate))
	switch action {
	case "favorite":
		return chatinfra.NewRepo(db).Set(t.Context(), 7, 3, true)
	case "create":
		_, err := tasks.Create(t.Context(), domain.Generation{UserID: 7, ConversationID: 3, Provider: "test", Model: "test"})
		return err
	case "start":
		return tasks.Start(t.Context(), 7, 1, 100)
	default:
		_, err := NewDoneWriter(db, gate).SaveDone(t.Context(), 7, 1, chatReply(), 100)
		return err
	}
}

// checkOrphans 删除后不得残留收藏、活动任务或未关联任务的助手消息。
func checkOrphans(t *testing.T, db *gorm.DB) {
	t.Helper()
	var count int64
	if err := db.Model(&model.Favorite{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("favorite count=%d err=%v", count, err)
	}
	var rows []model.Generation
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	messages := int64(0)
	for _, row := range rows {
		if row.Status != "cancelled" && row.Status != "completed" {
			t.Fatalf("active task after delete: %+v", row)
		}
		if row.Status == "completed" {
			messages++
		}
		if row.Status == "completed" && row.MessageID == nil {
			t.Fatal("completion lost message")
		}
	}
	checkDoneCounts(t, db, messages, messages)
}
