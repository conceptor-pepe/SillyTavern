// candidate.go 暴露记忆候选的提取、确认和拒绝接口。
package memoryhttp

import (
	"ai-chat/backend/internal/memory/domain"
	"ai-chat/backend/internal/reply"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) listCandidates(c *gin.Context) {
	uid, chatID, err := candidateIDs(c)
	if err != nil { // audit:allow-no-log 高频输入由固定错误码说明。
		candidateFail(c, err)
		return
	}
	items, err := h.service.ListCandidates(c.Request.Context(), uid, chatID)
	if err != nil { // audit:allow-no-log 应用层已记录。
		candidateFail(c, err)
		return
	}
	reply.OK(c, gin.H{"items": items})
}

func (h *Handler) extractCandidates(c *gin.Context) {
	uid, chatID, err := candidateIDs(c)
	if err != nil { // audit:allow-no-log 高频输入由固定错误码说明。
		candidateFail(c, err)
		return
	}
	var in struct {
		LeafID uint64 `json:"leaf_id,string"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if c.ShouldBindJSON(&in) != nil || in.LeafID == 0 { // audit:allow-no-log 高频输入由固定错误码说明。
		candidateFail(c, domain.ErrInvalid)
		return
	}
	items, err := h.service.ExtractCandidates(c.Request.Context(), uid, chatID, in.LeafID)
	if err != nil { // audit:allow-no-log 应用层已记录。
		candidateFail(c, err)
		return
	}
	reply.OK(c, gin.H{"items": items})
}

func (h *Handler) acceptCandidate(c *gin.Context) {
	uid, id, err := candidateIDs(c)
	if err != nil { // audit:allow-no-log 高频输入由固定错误码说明。
		candidateFail(c, err)
		return
	}
	out, err := h.service.AcceptCandidate(c.Request.Context(), uid, id)
	if err != nil { // audit:allow-no-log 应用层已记录。
		candidateFail(c, err)
		return
	}
	reply.OK(c, out)
}

func (h *Handler) rejectCandidate(c *gin.Context) {
	uid, id, err := candidateIDs(c)
	if err != nil { // audit:allow-no-log 高频输入由固定错误码说明。
		candidateFail(c, err)
		return
	}
	out, err := h.service.RejectCandidate(c.Request.Context(), uid, id)
	if err != nil { // audit:allow-no-log 应用层已记录。
		candidateFail(c, err)
		return
	}
	reply.OK(c, out)
}

func candidateIDs(c *gin.Context) (uint64, uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	uid := c.GetUint64("user_id")
	if err != nil || uid == 0 || id == 0 {
		// audit:allow-no-log 高频路径参数校验由固定错误码说明。
		return 0, 0, domain.ErrInvalid
	}
	return uid, id, nil
}

func candidateFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		reply.Fail(c, 400, "INVALID_MEMORY_CANDIDATE", "无法从当前分支整理记忆")
	case errors.Is(err, domain.ErrNotFound):
		reply.Fail(c, 404, "NOT_FOUND", "记忆候选不存在")
	case errors.Is(err, domain.ErrCandidateConflict):
		reply.Fail(c, 409, "CANDIDATE_CONFLICT", "记忆候选已经处理")
	default:
		reply.Fail(c, 502, "MEMORY_EXTRACTION_FAILED", "记忆整理暂不可用")
	}
}
