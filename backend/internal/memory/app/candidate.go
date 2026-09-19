// candidate.go 编排分支提取、候选审核和正式记忆写入。
package app

import (
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/memory/domain"
	msg "ai-chat/backend/internal/message/domain"
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

const candidateLimit = 45 * time.Second

// ExtractCandidates 从指定分支提出待用户确认的记忆。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @param leafID 分支叶节点
// @return 当前待确认候选与错误
func (s *Service) ExtractCandidates(ctx context.Context, uid, chatID, leafID uint64) ([]domain.Candidate, error) {
	repo, err := s.candidateRepo(uid)
	if err != nil {
		s.logger.Warn("candidate request rejected", zap.Uint64("user_id", uid), zap.Error(err))
		return nil, err
	}
	if chatID == 0 || leafID == 0 || s.extract == nil {
		s.logger.Warn("candidate extractor unavailable", zap.Uint64("user_id", uid))
		return nil, domain.ErrInvalid
	}
	scope, err := repo.CandidateScope(ctx, uid, chatID)
	if err != nil {
		s.logger.Warn("candidate scope failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return nil, err
	}
	branch, err := s.history.Branch(ctx, uid, chatID, leafID)
	if err != nil {
		s.logger.Warn("candidate branch failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return nil, err
	}
	if !candidateBranch(branch, chatID, leafID) {
		s.logger.Warn("candidate branch rejected", zap.Uint64("user_id", uid), zap.Uint64("chat_id", chatID))
		return nil, domain.ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, candidateLimit)
	defer cancel()
	proposals, err := s.extract.Extract(ctx, candidateTranscript(branch), scope.CompanionID != 0, scope.Story)
	if err != nil {
		s.logger.Warn("candidate extraction failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return nil, err
	}
	items := makeCandidates(scope, leafID, proposals)
	if len(items) == 0 {
		return repo.ListCandidates(ctx, uid, chatID)
	}
	out, err := repo.SaveCandidates(ctx, items)
	if err != nil {
		s.logger.Error("candidate save failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return nil, err
	}
	s.logger.Info("memory candidates extracted", zap.Uint64("user_id", uid), zap.Uint64("chat_id", chatID), zap.Int("count", len(items)))
	return out, nil
}

// ListCandidates 返回当前会话待确认候选。
// @param ctx 请求上下文
// @param uid 用户编号
// @param chatID 会话编号
// @return 候选列表与错误
func (s *Service) ListCandidates(ctx context.Context, uid, chatID uint64) ([]domain.Candidate, error) {
	repo, err := s.candidateRepo(uid)
	if err != nil {
		s.logger.Warn("candidate list rejected", zap.Uint64("user_id", uid), zap.Error(err))
		return nil, err
	}
	if chatID == 0 {
		return nil, domain.ErrInvalid
	}
	out, err := repo.ListCandidates(ctx, uid, chatID)
	if err != nil {
		s.logger.Warn("candidate list failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
	}
	return out, err
}

// AcceptCandidate 将候选原子写入其服务端确定的记忆范围。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 候选编号
// @return 候选结果与错误
func (s *Service) AcceptCandidate(ctx context.Context, uid, id uint64) (domain.Candidate, error) {
	repo, err := s.candidateRepo(uid)
	if err != nil {
		s.logger.Warn("candidate accept rejected", zap.Uint64("user_id", uid), zap.Error(err))
		return domain.Candidate{}, err
	}
	if id == 0 {
		return domain.Candidate{}, domain.ErrInvalid
	}
	out, err := repo.AcceptCandidate(ctx, uid, id)
	if err != nil {
		s.logger.Warn("candidate accept failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return out, err
	}
	s.logger.Info("memory candidate accepted", zap.Uint64("user_id", uid), zap.Uint64("candidate_id", id))
	return out, nil
}

// RejectCandidate 标记候选为拒绝，防止相同内容反复出现。
// @param ctx 请求上下文
// @param uid 用户编号
// @param id 候选编号
// @return 候选结果与错误
func (s *Service) RejectCandidate(ctx context.Context, uid, id uint64) (domain.Candidate, error) {
	repo, err := s.candidateRepo(uid)
	if err != nil {
		s.logger.Warn("candidate reject rejected", zap.Uint64("user_id", uid), zap.Error(err))
		return domain.Candidate{}, err
	}
	if id == 0 {
		return domain.Candidate{}, domain.ErrInvalid
	}
	out, err := repo.RejectCandidate(ctx, uid, id)
	if err != nil {
		s.logger.Warn("candidate reject failed", zap.Uint64("user_id", uid), zap.Error(logx.SafeError(err)))
		return out, err
	}
	s.logger.Info("memory candidate rejected", zap.Uint64("user_id", uid), zap.Uint64("candidate_id", id))
	return out, nil
}

func (s *Service) candidateRepo(uid uint64) (domain.CandidateRepo, error) {
	repo, ok := s.repo.(domain.CandidateRepo)
	if uid == 0 || !ok {
		return nil, domain.ErrInvalid
	}
	return repo, nil
}

func candidateBranch(items []msg.Message, chatID, leafID uint64) bool {
	if len(items) == 0 || items[len(items)-1].ID != leafID {
		return false
	}
	for _, item := range items {
		if item.ConversationID != chatID || item.Status != "completed" || (item.Role != "user" && item.Role != "assistant") {
			return false
		}
	}
	return true
}

func candidateTranscript(items []msg.Message) string {
	if len(items) > 24 {
		items = items[len(items)-24:]
	}
	var out strings.Builder
	for _, item := range items {
		out.WriteString("[" + item.Role + " #" + strconv.FormatUint(item.ID, 10) + "] " + item.Content + "\n")
	}
	return out.String()
}

func makeCandidates(scope domain.CandidateScope, leafID uint64, proposals []domain.Proposal) []domain.Candidate {
	out := make([]domain.Candidate, 0, len(proposals))
	for _, proposal := range proposals {
		sum := sha256.Sum256([]byte(proposal.Scope + "\x00" + strings.ToLower(proposal.Content)))
		out = append(out, domain.Candidate{UserID: scope.UserID, ChatID: scope.ChatID, CompanionID: scope.CompanionID,
			SourceEndID: leafID, Scope: proposal.Scope, Content: proposal.Content, Evidence: proposal.Evidence,
			Fingerprint: fmt.Sprintf("%x", sum)})
	}
	return out
}
