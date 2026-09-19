// background.go 提供账号背景 API，图片经过服务端解码重编码后才允许保存。
package preferencehttp

import (
	charapp "ai-chat/backend/internal/character/app"
	"ai-chat/backend/internal/logx"
	"ai-chat/backend/internal/preference/domain"
	"ai-chat/backend/internal/reply"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
)

// Handler 保存背景接口依赖。
type Handler struct {
	repo   domain.Repo
	logger *zap.Logger
}

// New 创建配置接口。
func New(repo domain.Repo, logger *zap.Logger) *Handler { return &Handler{repo: repo, logger: logger} }

// Routes 注册只操作当前账号的背景接口。
func (h *Handler) Routes(e *gin.Engine, auth gin.HandlerFunc) {
	e.GET("/api/v1/me/background", auth, h.get)
	e.PUT("/api/v1/me/background", auth, h.save)
}

// get 读取账号配置，前端不再以 localStorage 为权威来源。
func (h *Handler) get(c *gin.Context) {
	item, err := h.repo.Get(c.Request.Context(), c.GetUint64("user_id"))
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		h.fail(c, err)
		return
	}
	reply.OK(c, item)
}

// save 请求体与图片尺寸双重设限，拒绝 SVG 和远程图片 URL。
func (h *Handler) save(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 400000)
	var in domain.Background
	if err := c.ShouldBindJSON(&in); err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		reply.Fail(c, 400, "INVALID_INPUT", "背景格式无效")
		return
	}
	image, err := charapp.CleanPortrait(in.Image)
	if err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		reply.Fail(c, 400, "INVALID_IMAGE", "请选择较小的 JPG、PNG 或 WebP 图片")
		return
	}
	in.Image = image
	uid := c.GetUint64("user_id")
	if err := h.repo.Save(c.Request.Context(), uid, in); err != nil {
		// audit:allow-no-log 输入拒绝为高频校验；业务失败由应用服务或统一失败处理器记录。
		h.fail(c, err)
		return
	}
	h.logger.Info("background saved", zap.Uint64("user_id", uid))
	reply.OK(c, in)
}

// fail 不将存储错误或背景正文写入日志。
func (h *Handler) fail(c *gin.Context, err error) {
	h.logger.Error("background operation failed", zap.Uint64("user_id", c.GetUint64("user_id")), zap.Error(logx.SafeError(err)))
	reply.Fail(c, 500, "STORAGE_ERROR", "背景暂时无法保存或读取")
}
