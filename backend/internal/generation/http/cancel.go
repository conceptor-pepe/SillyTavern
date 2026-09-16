// cancel.go 负责生成取消入口，持久化授权成功后才允许中断本机 Provider。
package generationhttp

import (
	"errors"
	"net/http"

	"ai-chat/backend/internal/generation/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// cancel 先完成带用户和前置状态约束的更新，避免越权请求产生取消副作用。
func (h *Handler) cancel(c *gin.Context) {
	uid, id, err := readIDs(c)
	if err != nil {
		h.logger.Warn("generation cancel invalid", zap.String("request_id", c.GetString("request_id")),
			zap.String("generation_id", c.Param("id")), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_QUERY", "message": "invalid query"})
		return
	}
	if err := h.tasks.Cancel(c.Request.Context(), uid, id); err != nil {
		h.cancelError(c, uid, id, err)
		return
	}
	h.runner.Cancel(id)
	h.logger.Info("generation cancelled", zap.Uint64("user_id", uid),
		zap.Uint64("generation_id", id), zap.String("request_id", c.GetString("request_id")))
	c.Status(http.StatusNoContent)
}

// cancelError 区分不可取消任务和存储故障，不暴露任务归属或内部错误详情。
func (h *Handler) cancelError(c *gin.Context, uid, id uint64, err error) {
	fields := []zap.Field{zap.Uint64("user_id", uid), zap.Uint64("generation_id", id),
		zap.String("request_id", c.GetString("request_id")), zap.Error(err)}
	if errors.Is(err, domain.ErrNotFound) {
		h.logger.Warn("generation cancel rejected", fields...)
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "generation not cancellable"})
		return
	}
	h.logger.Error("generation cancel failed", fields...)
	c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "internal error"})
}
