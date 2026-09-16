// done_mysql_test.go 验证真实 MySQL 的生成完成事务，覆盖候选写入、终态冲突和回滚。
package infra

import (
	"testing"

	chatinfra "ai-chat/backend/internal/chat/infra"
	msgdomain "ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"gorm.io/gorm"
)

// TestDoneMySQL 验证主消息、候选、任务状态一起提交，重复完成不能多写消息。
func TestDoneMySQL(t *testing.T) {
	db := seedDone(t, "running")
	writer := NewDoneWriter(db, chatinfra.NewGate(db))
	item := msgdomain.Message{ConversationID: 3, Role: "assistant", Content: "A", Status: "completed",
		Variants: []msgdomain.Variant{{VariantNo: 0, Content: "A"}, {VariantNo: 1, Content: "B"}}}
	saved, err := writer.SaveDone(t.Context(), 7, 1, item, 100)
	if err != nil || saved.ID == 0 || len(saved.Variants) != 2 {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	for _, variant := range saved.Variants {
		if variant.MessageID != saved.ID {
			t.Fatalf("candidate bound to wrong message: %+v", variant)
		}
	}
	var task model.Generation
	if err := db.WithContext(t.Context()).First(&task, 1).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != "completed" || task.MessageID == nil || *task.MessageID != saved.ID {
		t.Fatalf("task=%+v", task)
	}
	if _, err := writer.SaveDone(t.Context(), 7, 1, item, 101); err == nil {
		t.Fatal("duplicate completion accepted")
	}
	checkDoneCounts(t, db, 1, 2)
}

// TestDoneRollback 验证终态、归属和候选校验失败时所有新增记录回滚。
func TestDoneRollback(t *testing.T) {
	for _, scenario := range []struct {
		name, status string
		uid          uint64
		variants     []msgdomain.Variant
	}{
		{name: "cancelled", status: "cancelled", uid: 7},
		{name: "foreign user", status: "running", uid: 8},
		{name: "duplicate candidate", status: "running", uid: 7,
			variants: []msgdomain.Variant{{VariantNo: 1}, {VariantNo: 1}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := seedDone(t, scenario.status)
			item := msgdomain.Message{ConversationID: 3, Role: "assistant", Content: "A",
				Status: "completed", Variants: scenario.variants}
			if _, err := NewDoneWriter(db, chatinfra.NewGate(db)).SaveDone(t.Context(), scenario.uid, 1, item, 100); err == nil {
				t.Fatal("invalid completion accepted")
			}
			checkDoneCounts(t, db, 0, 0)
			var task model.Generation
			if err := db.WithContext(t.Context()).First(&task, 1).Error; err != nil {
				t.Fatal(err)
			}
			if task.Status != scenario.status || task.MessageID != nil {
				t.Fatalf("rollback changed task: %+v", task)
			}
		})
	}
}

// seedDone 为每个场景建立独立任务和消息表，不复用可能污染的测试状态。
func seedDone(t *testing.T, status string) *gorm.DB {
	t.Helper()
	db := testdb.Open(t).WithContext(t.Context())
	if err := db.AutoMigrate(&model.Conversation{}, &model.Favorite{}, &model.Message{}, &model.MessageVariant{}, &model.Generation{}); err != nil {
		t.Fatal(err)
	}
	chat := model.Conversation{Base: model.Base{ID: 3}, UserID: 7, CharacterID: 1, ExtraData: "{}"}
	if err := db.Create(&chat).Error; err != nil {
		t.Fatal(err)
	}
	task := model.Generation{Base: model.Base{ID: 1}, UserID: 7, ConversationID: 3,
		Provider: "test", Model: "test", Status: status}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

// checkDoneCounts 对主消息和候选同时计数，防止仅检查任务状态漏掉孤立记录。
func checkDoneCounts(t *testing.T, db *gorm.DB, messages, variants int64) {
	t.Helper()
	for _, check := range []struct {
		table string
		count int64
	}{{"messages", messages}, {"message_variants", variants}} {
		var count int64
		if err := db.WithContext(t.Context()).Table(check.table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != check.count {
			t.Fatalf("%s count=%d want=%d", check.table, count, check.count)
		}
	}
}
