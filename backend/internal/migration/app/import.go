// import.go 负责把旧 JSONL 聊天记录转换为新会话和消息。
package app

import (
	"context"
	"errors"
	"path/filepath"

	"ai-chat/backend/internal/chat/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	legacy "ai-chat/backend/internal/migration/domain"
)

// Reader 读取旧聊天文件中的消息。
type Reader interface {
	Read(path string) ([]legacy.Message, error)
}

// ChatWriter 创建或查询迁移目标会话。
type ChatWriter interface {
	FindByTitle(ctx context.Context, userID uint64, title string) (domain.Conversation, error)
	Create(ctx context.Context, item domain.Conversation) (domain.Conversation, error)
}

// MessageWriter 写入迁移目标消息。
type MessageWriter interface {
	Create(ctx context.Context, userID uint64, item msgdomain.Message) (msgdomain.Message, error)
}

// BatchWriter 以单事务写入一个旧聊天文件。
type BatchWriter interface {
	ImportBatch(ctx context.Context, userID, characterID uint64, title string, items []legacy.Message) (bool, int, error)
}

// CharacterWriter 写入旧角色卡并返回是否新建。
type CharacterWriter interface {
	ImportCharacter(ctx context.Context, userID uint64, item legacy.Character, path string) (bool, error)
}

// UserWriter 写入旧用户账号并返回是否新建。
type UserWriter interface {
	ImportUser(ctx context.Context, item legacy.User) (bool, uint64, error)
}

// ImportCharacters 批量导入角色卡并统计新建和跳过数量。
func ImportCharacters(ctx context.Context, userID uint64, files []string, reader func(string) (legacy.Character, error), writer CharacterWriter) (int, int, []string) {
	created, skipped := 0, 0
	var failures []string
	for _, path := range files {
		item, err := reader(path)
		if err != nil {
			failures = append(failures, path+": "+err.Error())
			continue
		}
		ok, err := writer.ImportCharacter(ctx, userID, item, path)
		if err != nil {
			failures = append(failures, path+": "+err.Error())
			continue
		}
		if ok {
			created++
			continue
		}
		skipped++
	}
	return created, skipped, failures
}

// Report 保存一次迁移的结果统计。
type Report struct {
	Files       int
	SourceItems int
	Chats       int
	Written     int
	Skipped     int
	Errors      []string
}

// Import 批量读取旧文件并写入新会话消息。
func Import(ctx context.Context, userID, characterID uint64, files []string, reader Reader, chats ChatWriter, messages MessageWriter) Report {
	report := Report{}
	for _, path := range files {
		report.Files++
		if batch, ok := chats.(BatchWriter); ok {
			items, err := reader.Read(path)
			if err != nil {
				report.Errors = append(report.Errors, path+": "+err.Error())
				continue
			}
			report.SourceItems += len(items)
			created, count, err := batch.ImportBatch(ctx, userID, characterID, filepath.Base(path), items)
			if err != nil {
				report.Errors = append(report.Errors, path+": "+err.Error())
				continue
			}
			if created {
				report.Chats++
				report.Written += count
			} else {
				report.Skipped++
			}
			continue
		}
		if err := importFile(ctx, userID, characterID, path, reader, chats, messages, &report); err != nil {
			report.Errors = append(report.Errors, path+": "+err.Error())
		}
	}
	return report
}

// importFile 迁移单个文件，单文件失败不影响其他文件。
func importFile(ctx context.Context, userID, characterID uint64, path string, reader Reader, chats ChatWriter, messages MessageWriter, report *Report) error {
	items, err := reader.Read(path)
	if err != nil {
		return err
	}
	report.SourceItems += len(items)
	title := filepath.Base(path)
	chat, err := chats.FindByTitle(ctx, userID, title)
	existing := err == nil
	if errors.Is(err, legacy.ErrMissing) {
		chat, err = chats.Create(ctx, domain.Conversation{
			UserID: userID, CharacterID: characterID, Title: title, Status: "active",
		})
	}
	if err != nil && !existing {
		return err
	}
	if err != nil {
		return err
	}
	if chat.ID == 0 {
		return errors.New("migration chat id is empty")
	}
	if existing {
		report.Skipped++
		return nil
	}
	report.Chats++
	for _, item := range items {
		_, err = messages.Create(ctx, userID, msgdomain.Message{
			ConversationID: chat.ID, Role: item.Role, Content: item.Content,
			Status: "completed", ExtraData: item.ExtraData,
		})
		if err != nil {
			return err
		}
		report.Written++
	}
	return nil
}
