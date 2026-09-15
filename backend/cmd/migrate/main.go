// main.go 负责执行旧 JSONL 聊天到 MySQL 的一次性迁移。
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"ai-chat/backend/internal/config"
	"ai-chat/backend/internal/db"
	"ai-chat/backend/internal/logx"
	migrationapp "ai-chat/backend/internal/migration/app"
	migrationinfra "ai-chat/backend/internal/migration/infra"
	"go.uber.org/zap"
)

// checkArgs 保存迁移校验输入，避免校验函数参数过多。
type checkArgs struct {
	userID, characterID uint64
	files, cards        int
	report              migrationapp.Report
}

// main 读取迁移配置并执行聊天文件导入。
func main() {
	cfg := config.Load()
	logger, err := logx.New()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	root := required("AI_CHAT_DATA_ROOT")
	if os.Getenv("AI_CHAT_MIGRATE_DRY_RUN") == "1" {
		dryRun(root, logger)
		return
	}
	userID := optionalNumber("AI_CHAT_MIGRATE_USER_ID")
	characterID := number("AI_CHAT_MIGRATE_CHARACTER_ID")
	if cfg.MySQLDSN == "" {
		logger.Fatal("migration config missing", zap.String("key", "AI_CHAT_MYSQL_DSN"))
	}
	run(root, cfg.MySQLDSN, userID, characterID, logger)
}

// dryRun 只读取旧文件并输出迁移前的数据质量报告。
func dryRun(root string, logger *zap.Logger) {
	files, err := migrationinfra.ScanChats(root)
	if err != nil {
		logger.Fatal("migration files scan failed", zap.Error(err))
	}
	total := 0
	errorsCount := 0
	for _, file := range files {
		items, readErr := migrationinfra.ReadChat(file.Path)
		if readErr != nil {
			errorsCount++
			logger.Error("migration file invalid", zap.String("path", file.Path), zap.Error(readErr))
			continue
		}
		total += len(items)
	}
	logger.Info("migration dry run finished", zap.Int("files", len(files)),
		zap.Int("messages", total), zap.Int("errors", errorsCount))
	dryCards(root, logger)
	dryUsers(root, logger)
}

// dryCards 只解析角色卡并输出数据质量统计。
func dryCards(root string, logger *zap.Logger) {
	cardRoot := filepath.Join(filepath.Dir(root), "characters")
	files, err := migrationinfra.ListCards(cardRoot)
	if err != nil {
		logger.Error("character files scan failed", zap.String("path", cardRoot), zap.Error(err))
		return
	}
	errorsCount := 0
	for _, path := range files {
		if _, err := migrationinfra.ReadCard(path); err != nil {
			errorsCount++
			logger.Error("character card invalid", zap.String("path", path), zap.Error(err))
		}
	}
	logger.Info("character dry run finished", zap.Int("files", len(files)),
		zap.Int("errors", errorsCount))
}

// dryUsers 校验与正式迁移相同的来源账号匹配规则。
func dryUsers(root string, logger *zap.Logger) {
	item, err := sourceUser(root)
	if err != nil {
		logger.Error("migration source user invalid", zap.String("path", root), zap.Error(err))
		return
	}
	logger.Info("user dry run finished", zap.String("handle", item.Handle),
		zap.Int("matched", 1), zap.Int("errors", 0))
}

// run 连接数据库、扫描文件并输出迁移统计。
func run(root, dsn string, userID, characterID uint64, logger *zap.Logger) {
	ctx := context.Background()
	conn, err := db.OpenMySQL(ctx, dsn)
	if err != nil {
		logger.Fatal("migration database open failed", zap.Error(err))
	}
	if err := db.Migrate(ctx, conn); err != nil {
		logger.Fatal("migration database setup failed", zap.Error(err))
	}
	files, err := migrationinfra.ScanChats(root)
	if err != nil {
		logger.Fatal("migration files scan failed", zap.Error(err))
	}
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	writer := migrationinfra.NewGormWriter(conn)
	userID = importUser(ctx, root, userID, writer, logger)
	if userID == 0 {
		logger.Fatal("migration user missing")
	}
	if err := writer.CheckUser(ctx, userID); err != nil {
		logger.Fatal("migration target user invalid", zap.Uint64("user_id", userID), zap.Error(err))
	}
	cardCreated, cardSkipped := importCards(ctx, root, userID, writer, logger)
	report := migrationapp.Import(ctx, userID, characterID, paths,
		migrationinfra.LegacyReaderAdapter(migrationinfra.ReadChat), writer,
		migrationinfra.MessageWriterAdapter{Writer: writer})
	logger.Info("migration finished", zap.Int("files", report.Files),
		zap.Int("source_items", report.SourceItems), zap.Int("chats", report.Chats),
		zap.Int("written", report.Written), zap.Int("skipped", report.Skipped),
		zap.Int("errors", len(report.Errors)))
	for _, item := range report.Errors {
		logger.Error("migration item failed", zap.String("item", item))
	}
	check, err := checkData(ctx, writer, checkArgs{
		userID: userID, characterID: characterID, files: len(paths),
		cards: cardCreated + cardSkipped, report: report,
	})
	if err != nil {
		logger.Error("migration check failed", zap.Error(err))
		return
	}
	logCheck(logger, check)
}

// checkData 执行迁移后的目标数据校验。
func checkData(ctx context.Context, writer *migrationinfra.GormWriter, args checkArgs) (migrationinfra.CheckReport, error) {
	return writer.Check(ctx, migrationinfra.CheckInput{
		UserID: args.userID, SourceChats: args.files, SourceMsgs: args.report.SourceItems,
		SourceCards: args.cards, CharacterID: args.characterID,
	})
}

// logCheck 输出迁移校验报告。
func logCheck(logger *zap.Logger, check migrationinfra.CheckReport) {
	logger.Info("migration check finished", zap.Int("source_chats", check.SourceChats),
		zap.Int64("target_chats", check.TargetChats), zap.Int("source_messages", check.SourceMsgs),
		zap.Int64("target_messages", check.TargetMsgs), zap.Int("source_characters", check.SourceCards),
		zap.Int64("target_characters", check.TargetCards), zap.Int64("orphan_messages", check.OrphanMsgs),
		zap.Int64("invalid_chats", check.BadChats), zap.Int64("invalid_characters", check.BadCards),
		zap.Int("diffs", len(check.Diffs)))
}

// importCards 扫描并导入当前用户目录下的 PNG 角色卡。
func importCards(ctx context.Context, root string, userID uint64, writer *migrationinfra.GormWriter, logger *zap.Logger) (int, int) {
	cardRoot := filepath.Join(filepath.Dir(root), "characters")
	files, err := migrationinfra.ListCards(cardRoot)
	if err != nil {
		logger.Error("character files scan failed", zap.String("path", cardRoot), zap.Error(err))
		return 0, 0
	}
	created, skipped, failures := migrationapp.ImportCharacters(ctx, userID, files,
		migrationinfra.ReadCard, writer)
	logger.Info("character migration finished", zap.Int("files", len(files)),
		zap.Int("created", created), zap.Int("skipped", skipped),
		zap.Int("errors", len(failures)))
	for _, item := range failures {
		logger.Error("character migration failed", zap.String("item", item))
	}
	return created, skipped
}

// required 读取必须存在的迁移配置。
func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(errors.New(key + " is required"))
	}
	return value
}

// number 读取正整数迁移配置。
func number(key string) uint64 {
	value := required(key)
	result, err := strconv.ParseUint(value, 10, 64)
	if err != nil || result == 0 {
		panic(errors.New(key + " must be positive"))
	}
	return result
}

// optionalNumber 读取可选的正整数迁移配置。
func optionalNumber(key string) uint64 {
	value := os.Getenv(key)
	if value == "" {
		return 0
	}
	result, err := strconv.ParseUint(value, 10, 64)
	if err != nil || result == 0 {
		panic(errors.New(key + " must be positive"))
	}
	return result
}
