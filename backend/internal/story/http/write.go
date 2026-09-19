// write.go 请求只能指定内容和预期修订，不能指定作者或发布状态。
package storyhttp

import (
	"ai-chat/backend/internal/reply"
	"ai-chat/backend/internal/story/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (h *Handler) save(c *gin.Context) {
	var in struct {
		Definition domain.Definition `json:"definition"`
		Revision   uint64            `json:"expected_revision,string"`
	}
	if err := bind(c, &in); err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	item := domain.Story{UserID: c.GetUint64("user_id"), Revision: in.Revision, Definition: in.Definition}
	if c.Param("id") != "" {
		id, err := readID(c)
		if err != nil { // audit:allow-no-log 高频输入由错误码说明。
			fail(c, err)
			return
		}
		item.ID = id
	}
	out, err := h.service.Save(c.Request.Context(), item)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	h.logger.Info("story write completed", zap.Uint64("user_id", c.GetUint64("user_id")))
	reply.OK(c, out)
}

func (h *Handler) freeze(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	var in struct {
		Revision uint64 `json:"expected_revision,string"`
	}
	if err := bind(c, &in); err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.Freeze(c.Request.Context(), domain.Story{ID: id, UserID: c.GetUint64("user_id"), Revision: in.Revision})
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	h.logger.Info("story write completed", zap.Uint64("user_id", c.GetUint64("user_id")))
	reply.OK(c, out)
}

func (h *Handler) remove(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.GetUint64("user_id"), id); err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	h.logger.Info("story delete completed", zap.Uint64("user_id", c.GetUint64("user_id")))
	reply.OK(c, gin.H{"deleted": true})
}

func (h *Handler) publish(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	var in struct {
		Revision uint64 `json:"expected_revision,string"`
	}
	if err := bind(c, &in); err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.Publish(c.Request.Context(), domain.Story{
		ID: id, UserID: c.GetUint64("user_id"), Revision: in.Revision,
	})
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	h.logger.Info("story publication completed", zap.Uint64("user_id", c.GetUint64("user_id")), zap.Uint64("story_id", id))
	reply.OK(c, out)
}

func (h *Handler) unpublish(c *gin.Context) {
	id, err := readID(c)
	if err != nil { // audit:allow-no-log 高频输入由错误码说明。
		fail(c, err)
		return
	}
	out, err := h.service.Unpublish(c.Request.Context(), c.GetUint64("user_id"), id)
	if err != nil { // audit:allow-no-log 应用层已记录。
		fail(c, err)
		return
	}
	h.logger.Info("story unpublish completed", zap.Uint64("user_id", c.GetUint64("user_id")), zap.Uint64("story_id", id))
	reply.OK(c, out)
}
