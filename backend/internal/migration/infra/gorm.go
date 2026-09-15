// gorm.go 负责把旧聊天记录写入 MySQL 的会话和消息表。
package infra

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"ai-chat/backend/internal/chat/domain"
	msgdomain "ai-chat/backend/internal/message/domain"
	legacy "ai-chat/backend/internal/migration/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// GormWriter 使用 GORM 执行迁移目标写入。
type GormWriter struct {
	db *gorm.DB
}

// CheckUser 校验目标用户存在且处于启用状态。
func (w *GormWriter) CheckUser(ctx context.Context, userID uint64) error {
	if userID == 0 {
		return errors.New("migration user id is empty")
	}
	var row model.User
	err := w.db.WithContext(ctx).Where("id = ? AND enabled = ?", userID, true).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("migration target user is unavailable")
	}
	return err
}

// ImportUser 按账号幂等创建迁移用户，密码由新系统重新设置。
func (w *GormWriter) ImportUser(ctx context.Context, item legacy.User) (bool, uint64, error) {
	var found model.User
	err := w.db.WithContext(ctx).Where("handle = ?", item.Handle).First(&found).Error
	if err == nil {
		return false, found.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, 0, err
	}
	row := model.User{Handle: item.Handle, Name: item.Name, PasswordHash: "", Enabled: item.Enabled, Version: 1}
	if err := w.db.WithContext(ctx).Create(&row).Error; err != nil {
		return false, 0, err
	}
	return true, row.ID, nil
}

// MessageWriterAdapter 将 GORM 迁移器适配为消息写入接口。
type MessageWriterAdapter struct {
	Writer *GormWriter
}

// Create 写入一条迁移消息。
func (a MessageWriterAdapter) Create(ctx context.Context, userID uint64, item msgdomain.Message) (msgdomain.Message, error) {
	return a.Writer.CreateMessage(ctx, userID, item)
}

// NewGormWriter 创建 GORM 迁移写入器。
func NewGormWriter(db *gorm.DB) *GormWriter {
	return &GormWriter{db: db}
}

// FindByTitle 查找当前用户下同名的迁移会话。
func (w *GormWriter) FindByTitle(ctx context.Context, userID uint64, title string) (domain.Conversation, error) {
	var row model.Conversation
	err := w.db.WithContext(ctx).Where("user_id = ? AND title = ?", userID, title).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Conversation{}, legacy.ErrMissing
		}
		return domain.Conversation{}, err
	}
	return toChat(row), nil
}

// Create 创建迁移会话。
func (w *GormWriter) Create(ctx context.Context, item domain.Conversation) (domain.Conversation, error) {
	row := model.Conversation{
		UserID: item.UserID, CharacterID: item.CharacterID, Title: item.Title,
		Status: item.Status, ExtraData: "{}",
	}
	if err := w.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Conversation{}, err
	}
	return toChat(row), nil
}

// ImportCharacter 按用户和角色名幂等写入旧角色卡。
func (w *GormWriter) ImportCharacter(ctx context.Context, userID uint64, item legacy.Character, path string) (bool, error) {
	var found model.Character
	err := w.db.WithContext(ctx).Where("user_id = ? AND name = ?", userID, item.Name).First(&found).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	tags, marshalErr := json.Marshal(item.Tags)
	if marshalErr != nil {
		return false, marshalErr
	}
	extra, err := cardExtra(item, path)
	if err != nil {
		return false, err
	}
	row := model.Character{
		UserID: userID, Name: item.Name, Description: item.Description,
		Personality: item.Personality, Scenario: item.Scenario,
		FirstMessage: item.FirstMessage, MessageSample: item.MessageSample,
		Creator: item.Creator, Tags: string(tags), ExtraData: extra,
	}
	if err := w.db.WithContext(ctx).Create(&row).Error; err != nil {
		return false, err
	}
	return true, nil
}

// cardExtra 组合角色卡来源和扩展数据，供迁移写入角色表。
func cardExtra(item legacy.Character, path string) (string, error) {
	data := map[string]any{"source_path": path, "avatar": filepath.Base(path)}
	var raw map[string]any
	if json.Unmarshal(item.ExtraData, &raw) == nil {
		data["card"] = raw
	}
	value, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(value), nil
}

// ListCards 返回目录下所有 PNG 角色卡路径。
func ListCards(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
			paths = append(paths, filepath.Join(root, entry.Name()))
		}
	}
	return paths, nil
}

// Create 写入迁移消息，并保留旧消息扩展字段。
func (w *GormWriter) CreateMessage(ctx context.Context, userID uint64, item msgdomain.Message) (msgdomain.Message, error) {
	owned, err := w.ownsChat(ctx, userID, item.ConversationID)
	if err != nil || !owned {
		return msgdomain.Message{}, errors.New("migration chat not found")
	}
	row := model.Message{
		ConversationID: item.ConversationID, ParentID: item.ParentID, Role: item.Role,
		Content: item.Content, Status: item.Status, VariantNo: item.VariantNo,
		ExtraData: extraData(item.ExtraData),
	}
	if err := w.db.WithContext(ctx).Create(&row).Error; err != nil {
		return msgdomain.Message{}, err
	}
	return msgdomain.Message{
		ID: row.ID, ConversationID: row.ConversationID, ParentID: row.ParentID,
		Role: row.Role, Content: row.Content, Status: row.Status, VariantNo: row.VariantNo,
		ExtraData: []byte(row.ExtraData),
	}, nil
}

// ImportBatch 在一个事务中创建会话和其全部消息。
func (w *GormWriter) ImportBatch(ctx context.Context, userID, characterID uint64, title string, items []legacy.Message) (bool, int, error) {
	created := false
	count := 0
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.Conversation
		err := tx.Where("user_id = ? AND title = ?", userID, title).First(&row).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = model.Conversation{UserID: userID, CharacterID: characterID, Title: title, Status: "active", ExtraData: "{}"}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		created = true
		for _, item := range items {
			message := model.Message{
				ConversationID: row.ID, Role: item.Role, Content: item.Content,
				Status: "completed", ExtraData: extraData(item.ExtraData),
			}
			if err := tx.Create(&message).Error; err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return created, count, err
}

// extraData 返回可直接写入 JSON 列的扩展字段。
func extraData(value json.RawMessage) string {
	if len(value) == 0 {
		return "{}"
	}
	if !json.Valid(value) {
		return "{}"
	}
	return string(value)
}

func (w *GormWriter) ownsChat(ctx context.Context, userID, chatID uint64) (bool, error) {
	var count int64
	err := w.db.WithContext(ctx).Model(&model.Conversation{}).
		Where("id = ? AND user_id = ?", chatID, userID).Count(&count).Error
	return count > 0, err
}

func toChat(row model.Conversation) domain.Conversation {
	last := row.LastMsgAt
	return domain.Conversation{
		ID: row.ID, UserID: row.UserID, CharacterID: row.CharacterID,
		Title: row.Title, Status: row.Status, LastMsgAt: last,
	}
}

// TouchChat 更新迁移会话的最近消息时间。
func (w *GormWriter) TouchChat(ctx context.Context, chatID uint64, at time.Time) error {
	value := at.Unix()
	return w.db.WithContext(ctx).Model(&model.Conversation{}).
		Where("id = ?", chatID).Updates(map[string]any{"last_msg_at": value}).Error
}

// LegacyReaderAdapter 把旧记录读取函数适配为迁移应用接口。
type LegacyReaderAdapter func(path string) ([]legacy.Message, error)

// Read 调用旧聊天读取函数。
func (r LegacyReaderAdapter) Read(path string) ([]legacy.Message, error) {
	return r(path)
}
