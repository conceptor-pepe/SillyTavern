// delete_mysql_test.go 验证真实数据库中的会话软删除、用户隔离及子消息可见性。
package infra

import (
	"errors"
	"testing"

	"ai-chat/backend/internal/chat/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	msginfra "ai-chat/backend/internal/message/infra"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"gorm.io/gorm"
)

// TestDeleteMySQL 删除后保留原始记录，但任何用户入口不能继续访问或改写会话。
func TestDeleteMySQL(t *testing.T) {
	db := testdb.Open(t)
	seedChats(t, db)
	repo := NewRepo(db)
	ctx := t.Context()
	if err := repo.Delete(ctx, 8, 1); err != nil {
		t.Fatal(err)
	}
	if owned, err := repo.Owns(ctx, 7, 1); err != nil || !owned {
		t.Fatalf("foreign delete changed ownership: owned=%v err=%v", owned, err)
	}
	for range 2 {
		if err := repo.Delete(ctx, 7, 1); err != nil {
			t.Fatal(err)
		}
	}
	var row model.Conversation
	if err := db.First(&row, 1).Error; err != nil || row.DeletedAt == nil || row.UserID != 7 {
		t.Fatalf("deleted fact missing: row=%+v err=%v", row, err)
	}
	checkHiddenChat(t, repo)
	messages := msginfra.NewRepo(db)
	if _, err := messages.Find(ctx, 7, 1); !errors.Is(err, msgdomain.ErrNotFound) {
		t.Fatalf("deleted chat message visible: %v", err)
	}
	items, total, err := messages.List(ctx, 7, 1, 1, 20)
	if err != nil || len(items) != 0 || total != 0 {
		t.Fatalf("deleted history: items=%v total=%d err=%v", items, total, err)
	}
	variants, total, err := messages.ListVariants(ctx, 7, 1, 1, 20)
	if err != nil || len(variants) != 0 || total != 0 {
		t.Fatalf("deleted variants: items=%v total=%d err=%v", variants, total, err)
	}
}

// seedChats 建立同用户活跃会话及其他用户会话，避免空库掩盖过滤错误。
func seedChats(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&model.Conversation{}, &model.Favorite{}, &model.Message{}, &model.MessageVariant{}); err != nil {
		t.Fatal(err)
	}
	last := int64(10)
	rows := []model.Conversation{
		{Base: model.Base{ID: 1}, UserID: 7, CharacterID: 1, Title: "deleted", Status: "active", LastMsgAt: &last, ExtraData: "{}"},
		{Base: model.Base{ID: 2}, UserID: 7, CharacterID: 1, Title: "visible", Status: "active", LastMsgAt: &last, ExtraData: "{}"},
		{Base: model.Base{ID: 3}, UserID: 8, CharacterID: 1, Title: "private", Status: "active", LastMsgAt: &last, ExtraData: "{}"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	message := model.Message{Base: model.Base{ID: 1}, ConversationID: 1, Role: "assistant", Content: "hidden", Status: "completed", ExtraData: "{}"}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	variant := model.MessageVariant{MessageID: 1, VariantNo: 1, Content: "hidden", ExtraData: "{}"}
	if err := db.Create(&variant).Error; err != nil {
		t.Fatal(err)
	}
}

// checkHiddenChat 检查列表、最近聊天、详情、归属、改名及收藏共用删除边界。
func checkHiddenChat(t *testing.T, repo *Repo) {
	t.Helper()
	ctx := t.Context()
	for _, load := range []func() ([]domain.Conversation, int64, error){
		func() ([]domain.Conversation, int64, error) { return repo.List(ctx, 7, 1, 20) },
		func() ([]domain.Conversation, int64, error) { return repo.Recent(ctx, 7, 1, 20) },
	} {
		items, total, err := load()
		if err != nil || total != 1 || len(items) != 1 || items[0].ID != 2 {
			t.Fatalf("visible chats: items=%v total=%d err=%v", items, total, err)
		}
	}
	if _, err := repo.Find(ctx, 7, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted detail: %v", err)
	}
	if owned, err := repo.Owns(ctx, 7, 1); err != nil || owned {
		t.Fatalf("deleted ownership: owned=%v err=%v", owned, err)
	}
	if err := repo.UpdateTitle(ctx, 7, 1, "revived"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted rename: %v", err)
	}
	if err := repo.Set(ctx, 7, 1, true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted favorite: %v", err)
	}
}
