// handler.go 公开作者私有故事 API，不暴露公共发布入口。
package storyhttp

import (
	"ai-chat/backend/internal/reply"
	"ai-chat/backend/internal/story/app"
	"ai-chat/backend/internal/story/domain"
	"errors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

// Handler 将输入归属固定为认证用户。
type Handler struct {
	service *app.Service
	logger  *zap.Logger
}

// New 创建故事接口。
// @param service 作品服务
// @param logger 业务日志
// @return 故事接口
func New(service *app.Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// Routes 注册私有草稿与版本路由。
// @param e 路由引擎
// @param auth 认证中间件
func (h *Handler) Routes(e *gin.Engine, auth gin.HandlerFunc) {
	e.GET("/api/v1/public/stories", h.publicList)
	e.GET("/api/v1/public/stories/:id", h.publicFind)
	e.GET("/api/v1/stories", auth, h.list)
	e.POST("/api/v1/stories", auth, h.save)
	e.GET("/api/v1/stories/:id", auth, h.find)
	e.PUT("/api/v1/stories/:id", auth, h.save)
	e.DELETE("/api/v1/stories/:id", auth, h.remove)
	e.POST("/api/v1/stories/:id/versions", auth, h.freeze)
	e.POST("/api/v1/stories/:id/publication", auth, h.publish)
	e.DELETE("/api/v1/stories/:id/publication", auth, h.unpublish)
	e.GET("/api/v1/story-versions/:id", auth, h.version)
}

func readID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		// audit:allow-no-log 高频路径参数校验以固定错误码返回。
		return 0, domain.ErrInvalid
	}
	return id, nil
}

func bind(c *gin.Context, in any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	if c.ShouldBindJSON(in) != nil {
		return domain.ErrInvalid
	}
	return nil
}

func fail(c *gin.Context, err error) {
	status, code := 500, "STORAGE_ERROR"
	switch {
	case errors.Is(err, domain.ErrInvalid):
		status, code = 400, "INVALID_STORY"
	case errors.Is(err, domain.ErrNotFound):
		status, code = 404, "NOT_FOUND"
	case errors.Is(err, domain.ErrConflict):
		status, code = 409, "REVISION_CONFLICT"
	}
	reply.Fail(c, status, code, http.StatusText(status))
}
