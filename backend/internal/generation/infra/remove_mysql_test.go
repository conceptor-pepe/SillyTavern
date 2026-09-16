// remove_mysql_test.go 验证会话删除跨域事务的终态保护、幂等和失败回滚。
package infra

import (
	"context"
	"errors"
	"reflect"
	"testing"

	chatapp "ai-chat/backend/internal/chat/app"
	chatdomain "ai-chat/backend/internal/chat/domain"
	chatinfra "ai-chat/backend/internal/chat/infra"
	genapp "ai-chat/backend/internal/generation/app"
	"ai-chat/backend/internal/generation/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/port"
	"gorm.io/gorm"
)

// TestRemoveTasks 删除仅取消活动任务，保留其他任务全部事实和无关收藏。
func TestRemoveTasks(t *testing.T) {
	for _, status := range []string{"pending", "running", "completed", "failed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			db := seedDone(t, status)
			seedFavorites(t, db)
			before := loadTask(t, db)
			gate := chatinfra.NewGate(db)
			tasks := genapp.New(NewRepo(db, gate))
			remove := chatapp.NewRemover(chatinfra.NewRepo(db), gate, tasks)
			if err := remove.Delete(t.Context(), 8, 3); err != nil {
				t.Fatal(err)
			}
			checkRemoval(t, db, false)
			for range 2 {
				if err := remove.Delete(t.Context(), 7, 3); err != nil {
					t.Fatal(err)
				}
			}
			checkRemoval(t, db, true)
			after := loadTask(t, db)
			if status == "pending" || status == "running" {
				if after.Status != "cancelled" || after.FinishedAt == nil || after.UserID != 7 || after.MessageID != nil {
					t.Fatalf("invalid cancellation: %+v", after)
				}
			} else if !reflect.DeepEqual(before, after) {
				t.Fatalf("terminal changed: before=%+v after=%+v", before, after)
			}
			if _, err := tasks.Find(t.Context(), 7, 1); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("deleted task visible: %v", err)
			}
			if err := tasks.Start(t.Context(), 7, 1, 200); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("deleted task started: %v", err)
			}
		})
	}
}

// brokenCancel 在真实取消写入后返回错误，证明三个业务事实属于同一事务。
type brokenCancel struct{ port.ChatTasks }

// CancelChat 故意使用领域不存在错误，验证回调错误不会被误认为幂等删除。
func (b brokenCancel) CancelChat(ctx context.Context, uid, id uint64) error {
	if err := b.ChatTasks.CancelChat(ctx, uid, id); err != nil {
		return err
	}
	return chatdomain.ErrNotFound
}

// TestRemoveRollback 任务取消后的错误必须恢复会话、收藏及任务原始事实。
func TestRemoveRollback(t *testing.T) {
	db := seedDone(t, "running")
	seedFavorites(t, db)
	before := loadTask(t, db)
	gate := chatinfra.NewGate(db)
	tasks := genapp.New(NewRepo(db, gate))
	remove := chatapp.NewRemover(chatinfra.NewRepo(db), gate, brokenCancel{tasks})
	if err := remove.Delete(t.Context(), 7, 3); err == nil {
		t.Fatal("cancellation failure ignored")
	}
	checkRemoval(t, db, false)
	if after := loadTask(t, db); !reflect.DeepEqual(before, after) {
		t.Fatalf("task escaped rollback: %+v", after)
	}
}

// seedFavorites 准备目标会话收藏及不同类型、不同用户的无关收藏。
func seedFavorites(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []model.Favorite{{UserID: 7, CharacterID: 3, Kind: "chat"},
		{UserID: 7, CharacterID: 3, Kind: "character"}, {UserID: 8, CharacterID: 4, Kind: "chat"}}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
}

// loadTask 直接读取持久化事实，避免公开可见性过滤掩盖错误状态。
func loadTask(t *testing.T, db *gorm.DB) model.Generation {
	t.Helper()
	var row model.Generation
	if err := db.First(&row, 1).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

// checkRemoval 同时检查软删除及收藏范围，防止误删角色或其他用户关系。
func checkRemoval(t *testing.T, db *gorm.DB, deleted bool) {
	t.Helper()
	var chat model.Conversation
	if err := db.First(&chat, 3).Error; err != nil || (chat.DeletedAt != nil) != deleted {
		t.Fatalf("chat=%+v err=%v", chat, err)
	}
	var rows []model.Favorite
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	want := 3
	if deleted {
		want = 2
	}
	if len(rows) != want || rows[len(rows)-2].Kind != "character" || rows[len(rows)-1].UserID != 8 {
		t.Fatalf("favorite facts changed: %+v", rows)
	}
}
