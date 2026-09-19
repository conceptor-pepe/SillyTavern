package db

import (
	"testing"

	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
)

// TestStoryMigrationMySQL 验证故事结构升级可重复执行且记录完成状态。
func TestStoryMigrationMySQL(t *testing.T) {
	conn := testdb.Open(t)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migration is not repeatable: %v", err)
	}
	migrator := conn.Migrator()
	for _, table := range []any{&model.Story{}, &model.StoryVersion{}, &model.PlayerPersona{}, &model.StorySession{}, &model.Relationship{}, &model.MemoryCandidate{}} {
		if !migrator.HasTable(table) {
			t.Fatalf("missing story table for %T", table)
		}
	}
	if !migrator.HasColumn(&model.Conversation{}, "mode") || !migrator.HasColumn(&model.Memory{}, "chat_id") {
		t.Fatal("story scope columns are missing")
	}
	if !migrator.HasColumn(&model.Story{}, "published_version_id") || !migrator.HasColumn(&model.Story{}, "published_at") {
		t.Fatal("story publication columns are missing")
	}
	if !migrator.HasColumn(&model.StorySession{}, "companion_id") || !migrator.HasIndex(&model.Relationship{}, "uk_user_companion") {
		t.Fatal("relationship scope is missing")
	}
	var row schemaChange
	if err := conn.Where("version = ?", storyVersion).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "applied" {
		t.Fatalf("unexpected migration state: %s", row.State)
	}
	var publication schemaChange
	if err := conn.Where("version = ? AND state = ?", publicationVersion, "applied").First(&publication).Error; err != nil {
		t.Fatal(err)
	}
	var relationship schemaChange
	if err := conn.Where("version = ? AND state = ?", relationshipVersion, "applied").First(&relationship).Error; err != nil {
		t.Fatal(err)
	}
	var candidate schemaChange
	if err := conn.Where("version = ? AND state = ?", memoryCandidateVersion, "applied").First(&candidate).Error; err != nil {
		t.Fatal(err)
	}
}
