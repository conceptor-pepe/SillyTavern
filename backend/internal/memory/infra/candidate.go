// candidate.go 持久化候选，并在事务中把用户确认转换为正式记忆。
package infra

import (
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/model"
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CandidateScope 从本人会话解析是否允许跨故事关系范围。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @return 候选范围与错误
func (r *Repo) CandidateScope(ctx context.Context, uid, chatID uint64) (domain.CandidateScope, error) {
	var chat model.Conversation
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ? AND deleted_at IS NULL", chatID, uid).First(&chat).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.CandidateScope{}, candidateError(err)
	}
	out := domain.CandidateScope{UserID: uid, ChatID: chatID, CompanionID: chat.CharacterID, Story: chat.Mode == "story"}
	if chat.Mode != "story" {
		return out, nil
	}
	var session model.StorySession
	if err := r.db.WithContext(ctx).Where("chat_id = ? AND user_id = ?", chatID, uid).First(&session).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return domain.CandidateScope{}, candidateError(err)
	}
	out.CompanionID = session.CompanionID
	return out, nil
}

// ListCandidates 返回当前会话未处理的候选。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @return 候选列表与错误
func (r *Repo) ListCandidates(ctx context.Context, uid, chatID uint64) ([]domain.Candidate, error) {
	if _, err := r.CandidateScope(ctx, uid, chatID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	var rows []model.MemoryCandidate
	err := r.db.WithContext(ctx).Where("user_id = ? AND chat_id = ? AND status = ? AND deleted_at IS NULL", uid, chatID, "pending").
		Order("id DESC").Limit(50).Find(&rows).Error
	return mapCandidates(rows), err
}

// SaveCandidates 幂等保存同一分支提议，已拒绝内容不会反复出现。
// @param ctx 请求上下文
// @param items 候选列表
// @return 当前待确认候选与错误
func (r *Repo) SaveCandidates(ctx context.Context, items []domain.Candidate) ([]domain.Candidate, error) {
	if len(items) == 0 {
		return []domain.Candidate{}, nil
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			row := candidateRow(item)
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
				return err
			}
		}
		return nil
	})
	if err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return nil, err
	}
	return r.ListCandidates(ctx, items[0].UserID, items[0].ChatID)
}

// AcceptCandidate 原子创建正式记忆并标记候选，重复确认不会重复写入。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 候选编号
// @return 候选结果与错误
func (r *Repo) AcceptCandidate(ctx context.Context, uid, id uint64) (domain.Candidate, error) {
	var candidate model.MemoryCandidate
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockCandidate(tx, uid, id, &candidate); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if candidate.Status != "pending" {
			return domain.ErrCandidateConflict
		}
		memory, err := acceptedMemory(tx, candidate)
		if err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		candidate.Status, candidate.MemoryID = "accepted", &memory.ID
		return tx.Model(&candidate).Select("status", "memory_id").Updates(&candidate).Error
	})
	return toCandidate(candidate), err
}

// RejectCandidate 永久拒绝候选，后续相同指纹不会重新出现。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 候选编号
// @return 候选结果与错误
func (r *Repo) RejectCandidate(ctx context.Context, uid, id uint64) (domain.Candidate, error) {
	var candidate model.MemoryCandidate
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockCandidate(tx, uid, id, &candidate); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return err
		}
		if candidate.Status != "pending" {
			return domain.ErrCandidateConflict
		}
		candidate.Status = "rejected"
		return tx.Model(&candidate).Update("status", candidate.Status).Error
	})
	return toCandidate(candidate), err
}

func lockCandidate(tx *gorm.DB, uid, id uint64, out *model.MemoryCandidate) error {
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, uid).First(out).Error
	return candidateError(err)
}

func acceptedMemory(tx *gorm.DB, candidate model.MemoryCandidate) (model.Memory, error) {
	row := model.Memory{UserID: candidate.UserID, Kind: "fact", Content: candidate.Content,
		Keywords: "[]", Enabled: true, Pinned: true, Embedding: "[]", EmbeddingModel: ""}
	if candidate.Scope == "relationship" {
		if err := lockRelationship(tx, candidate.UserID, candidate.CompanionID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return row, err
		}
		row.CharacterID = candidate.CompanionID
	} else {
		if err := lockChat(tx, candidate.UserID, candidate.ChatID); err != nil { // audit:allow-no-log 应用层记录仓储错误。
			return row, err
		}
		row.ChatID = candidate.ChatID
	}
	query := candidateMemoryQuery(tx, candidate)
	var existing model.Memory
	if err := query.Where("content = ?", candidate.Content).First(&existing).Error; err == nil {
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) { // audit:allow-no-log 应用层记录仓储错误。
		return row, err
	}
	// GORM 的 First 会把 record-not-found 保存在返回查询上，后续配额查询必须使用全新会话。
	query = candidateMemoryQuery(tx.Session(&gorm.Session{NewDB: true}), candidate)
	var count int64
	if err := query.Count(&count).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return row, err
	}
	if count >= 200 {
		return row, domain.ErrInvalid
	}
	var pinned int64
	if err := query.Where("enabled = ? AND pinned = ?", true, true).
		Select("COALESCE(SUM(OCTET_LENGTH(content) + 3), 0)").Scan(&pinned).Error; err != nil { // audit:allow-no-log 应用层记录仓储错误。
		return row, err
	}
	if pinned+int64(len(candidate.Content))+3 > 6000 {
		return row, domain.ErrInvalid
	}
	return row, tx.Create(&row).Error
}

func candidateMemoryQuery(tx *gorm.DB, candidate model.MemoryCandidate) *gorm.DB {
	query := tx.Model(&model.Memory{}).Where("user_id = ?", candidate.UserID)
	if candidate.Scope == "relationship" {
		return query.Where("character_id = ? AND chat_id = 0", candidate.CompanionID)
	}
	return query.Where("chat_id = ? AND character_id = 0", candidate.ChatID)
}

func candidateRow(item domain.Candidate) model.MemoryCandidate {
	return model.MemoryCandidate{UserID: item.UserID, ChatID: item.ChatID, CompanionID: item.CompanionID,
		SourceEndID: item.SourceEndID, Scope: item.Scope, Content: item.Content, Evidence: item.Evidence,
		Fingerprint: item.Fingerprint, Status: "pending"}
}

func mapCandidates(rows []model.MemoryCandidate) []domain.Candidate {
	out := make([]domain.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCandidate(row))
	}
	return out
}

func toCandidate(row model.MemoryCandidate) domain.Candidate {
	return domain.Candidate{ID: row.ID, UserID: row.UserID, ChatID: row.ChatID, CompanionID: row.CompanionID,
		SourceEndID: row.SourceEndID, Scope: row.Scope, Content: row.Content, Evidence: row.Evidence,
		Status: row.Status, MemoryID: row.MemoryID, Fingerprint: row.Fingerprint}
}

func candidateError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
