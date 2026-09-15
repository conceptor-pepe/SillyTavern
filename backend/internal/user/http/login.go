// login.go 提供用户登录 HTTP Handler 和路由注册。
package userhttp

import (
	"net/http"
	"time"

	"ai-chat/backend/internal/user/app"
	"ai-chat/backend/internal/user/auth"
	"ai-chat/backend/internal/user/domain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoginHandler 保存登录接口所需的用例和会话配置。
type LoginHandler struct {
	repo   domain.Repo
	secret string
	logger *zap.Logger
}

// NewLogin 创建登录 Handler。
func NewLogin(repo domain.Repo, secret string, logger *zap.Logger) *LoginHandler {
	return &LoginHandler{repo: repo, secret: secret, logger: logger}
}

// RegisterRoutes 注册登录公开路由。
func (h *LoginHandler) RegisterRoutes(engine *gin.Engine, auth gin.HandlerFunc) {
	engine.POST("/api/users/login", h.login)
	engine.POST("/api/users/register", h.register)
	engine.POST("/api/users/logout", auth, h.logout)
	engine.GET("/api/users/me", auth, h.me)
	engine.POST("/api/v1/auth/login", h.login)
	engine.POST("/api/v1/auth/register", h.register)
	engine.POST("/api/v1/auth/logout", auth, h.logout)
	engine.GET("/api/v1/me", auth, h.me)
}

// register 创建账号并设置登录 Cookie。
func (h *LoginHandler) register(c *gin.Context) {
	var in app.RegisterIn
	if err := c.ShouldBindJSON(&in); err != nil {
		h.logger.Warn("register request invalid", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	out, err := app.Register(c.Request.Context(), h.repo, in)
	if err != nil {
		h.logger.Warn("register failed", zap.String("handle", in.Handle), zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "register failed"})
		return
	}
	token := auth.Sign(h.secret, out.ID, out.Version, time.Now())
	c.SetCookie("ai_chat_token", token, 86400, "/", "", true, true)
	h.logger.Info("register succeeded", zap.Uint64("user_id", out.ID))
	c.JSON(http.StatusCreated, gin.H{"user": out})
}

// login 解析请求、执行登录并写入 HttpOnly Cookie。
func (h *LoginHandler) login(c *gin.Context) {
	var in app.LoginIn
	if err := c.ShouldBindJSON(&in); err != nil {
		h.logger.Warn("login request invalid", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	out, err := app.Login(c.Request.Context(), h.repo, in, app.MatchPass)
	if err != nil {
		h.logger.Warn("login failed", zap.String("handle", in.Handle), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login failed"})
		return
	}
	token := auth.Sign(h.secret, out.ID, out.Version, time.Now())
	c.SetCookie("ai_chat_token", token, 86400, "/", "", true, true)
	h.logger.Info("login succeeded", zap.Uint64("user_id", out.ID))
	c.JSON(http.StatusOK, gin.H{"user": out})
}

// logout 清理当前登录 Cookie。
func (h *LoginHandler) logout(c *gin.Context) {
	c.SetCookie("ai_chat_token", "", -1, "/", "", true, true)
	h.logger.Info("logout succeeded")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// me 返回令牌对应的当前用户。
func (h *LoginHandler) me(c *gin.Context) {
	id, ok := userID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.repo.FindID(c.Request.Context(), id)
	if err != nil || !user.Enabled {
		h.logger.Warn("me failed", zap.Uint64("user_id", id), zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": userView(user)})
}

// userID 从认证上下文读取用户编号。
func userID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}

// userView 删除密码字段后生成用户响应。
func userView(user domain.User) app.LoginOut {
	return app.LoginOut{ID: user.ID, Handle: user.Handle, Name: user.Name, Version: user.Version}
}
