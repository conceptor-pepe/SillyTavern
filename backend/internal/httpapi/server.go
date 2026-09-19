// server.go 负责创建 Gin 服务、基础中间件和健康检查。
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	charhttp "ai-chat/backend/internal/character/http"
	charinfra "ai-chat/backend/internal/character/infra"
	chatapp "ai-chat/backend/internal/chat/app"
	chathttp "ai-chat/backend/internal/chat/http"
	chatinfra "ai-chat/backend/internal/chat/infra"
	"ai-chat/backend/internal/chatcontext"
	"ai-chat/backend/internal/config"
	"ai-chat/backend/internal/db"
	genapp "ai-chat/backend/internal/generation/app"
	gendomain "ai-chat/backend/internal/generation/domain"
	genhttp "ai-chat/backend/internal/generation/http"
	geninfra "ai-chat/backend/internal/generation/infra"
	"ai-chat/backend/internal/logx"
	memapp "ai-chat/backend/internal/memory/app"
	memdomain "ai-chat/backend/internal/memory/domain"
	memhttp "ai-chat/backend/internal/memory/http"
	meminfra "ai-chat/backend/internal/memory/infra"
	msghttp "ai-chat/backend/internal/message/http"
	msginfra "ai-chat/backend/internal/message/infra"
	prefhttp "ai-chat/backend/internal/preference/http"
	prefinfra "ai-chat/backend/internal/preference/infra"
	providerinfra "ai-chat/backend/internal/provider/infra"
	relapp "ai-chat/backend/internal/relationship/app"
	relhttp "ai-chat/backend/internal/relationship/http"
	relinfra "ai-chat/backend/internal/relationship/infra"
	userhttp "ai-chat/backend/internal/user/http"
	userinfra "ai-chat/backend/internal/user/infra"
	"ai-chat/backend/internal/webui"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrClosed 表示 HTTP 服务已正常关闭。
var ErrClosed = http.ErrServerClosed

// Server 保存 Gin 引擎和 HTTP 服务实例。
type Server struct {
	http      *http.Server
	sql       *gorm.DB
	redis     *redis.Client
	cleaner   *taskCleaner
	drain     *requestDrain
	closeOnce sync.Once
	closeErr  error
}

// New 创建只包含基础能力的 API 服务。
func New(cfg config.Config, logger *zap.Logger) (*Server, error) {
	engine := gin.New()
	engine.Use(recoverRequest(logger), requestID())
	registerHealth(engine)
	engine.NoRoute(gin.WrapH(webui.Handler()))
	drain := newDrain()
	server := &Server{
		drain: drain,
		http: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           drain.wrap(engine),
			ReadHeaderTimeout: 5 * time.Second,
			ErrorLog:          zap.NewStdLog(logger),
		},
	}
	if err := server.connect(cfg, engine, logger); err != nil {
		zap.L().Error("server infrastructure failed", zap.Error(logx.SafeError(err)))
		drain.stop()
		return nil, errors.Join(err, server.closeClients())
	}
	return server, nil
}

// connect 装配存储和业务路由；任务补偿不依赖当前是否启用模型供应商。
func (s *Server) connect(cfg config.Config, engine *gin.Engine, logger *zap.Logger) error {
	if cfg.RedisAddr != "" {
		client, err := db.OpenRedis(context.Background(), cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
		if err != nil {
			zap.L().Error("server infrastructure failed", zap.Error(logx.SafeError(err)))
			return err
		}
		s.redis = client
		logger.Info("redis ready", zap.String("addr", cfg.RedisAddr))
	}
	if cfg.MySQLDSN != "" {
		conn, err := openDB(cfg, logger)
		if err != nil {
			zap.L().Error("server infrastructure failed", zap.Error(logx.SafeError(err)))
			return err
		}
		s.sql = conn
		registerModules(engine, conn, cfg, logger)
		memory := memoryService(conn, cfg, logger)
		memhttp.New(memory).Routes(engine, RequireAuth(cfg.AuthSecret))
		prefhttp.New(prefinfra.New(conn), logger).Routes(engine, RequireAuth(cfg.AuthSecret))
		relhttp.New(relapp.New(relinfra.New(conn), logger)).Routes(engine, RequireAuth(cfg.AuthSecret))
		gate := chatinfra.NewGate(conn)
		tasks := genapp.New(geninfra.NewRepo(conn, gate))
		if cfg.ProviderURL != "" {
			provider := &providerinfra.OpenAI{URL: cfg.ProviderURL, Key: cfg.ProviderKey}
			runner := genapp.NewRunner(tasks, provider, geninfra.NewDoneWriter(conn, gate))
			genhttp.New(tasks, runner, genhttp.Deps{
				Story:        chatcontext.New(chatinfra.NewStory(conn), memory, logger),
				DefaultModel: cfg.ProviderModel, Memory: memory, Suggester: genapp.NewSuggester(provider, logger), OutputTokens: cfg.OutputTokens,
				Chats: chatinfra.NewRepo(conn), Chars: charinfra.NewRepo(conn), Msgs: msginfra.NewRepo(conn),
			}, logger).Routes(engine, RequireAuth(cfg.AuthSecret))
			logger.Info("provider ready", zap.String("model", cfg.ProviderModel),
				zap.String("provider", "openai"))
		} else {
			logger.Info("provider disabled", zap.String("reason", "AI_CHAT_PROVIDER_URL is empty"),
				zap.String("status", gendomain.StatusPending))
		}
		s.cleaner = startCleaner(tasks, logger, time.Minute)
		logger.Info("mysql ready")
	}
	return nil
}

// registerModules 注册数据库相关的业务路由。
func registerModules(engine *gin.Engine, conn *gorm.DB, cfg config.Config, logger *zap.Logger) {
	auth := RequireAuth(cfg.AuthSecret)
	login := userhttp.NewLogin(userinfra.NewRepo(conn), cfg.AuthSecret, logger)
	if cfg.LocalHTTP() {
		login.UseLocalHTTP()
	}
	login.RegisterRoutes(engine, auth)
	charhttp.New(charinfra.NewRepo(conn), logger).Routes(engine, auth)
	chats := chatinfra.NewRepo(conn)
	gate := chatinfra.NewGate(conn)
	tasks := genapp.New(geninfra.NewRepo(conn, gate))
	remove := chatapp.NewRemover(chats, gate, tasks)
	handler := chathttp.New(chats, charinfra.NewRepo(conn), remove, logger, chats)
	handler.UseStory(chatapp.NewStory(chatinfra.NewStory(conn), logger))
	handler.Routes(engine, auth)
	registerStory(engine, conn, auth, logger)
	msghttp.New(msginfra.NewRepo(conn), chatinfra.NewRepo(conn), logger).Routes(engine, auth)
}

// openDB 创建 MySQL 连接并执行结构迁移。
func openDB(cfg config.Config, logger *zap.Logger) (*gorm.DB, error) {
	conn, err := db.OpenMySQL(context.Background(), cfg.MySQLDSN, logger)
	if err != nil {
		zap.L().Error("server infrastructure failed", zap.Error(logx.SafeError(err)))
		return nil, err
	}
	if err := db.Migrate(context.Background(), conn); err != nil {
		zap.L().Error("server infrastructure failed", zap.Error(logx.SafeError(err)))
		return nil, closeDB(conn, err)
	}
	return conn, nil
}

// closeDB 关闭迁移失败时的数据库连接，并保留原始错误。
func closeDB(conn *gorm.DB, cause error) error {
	sqlDB, err := conn.DB()
	if err != nil {
		zap.L().Error("server infrastructure failed", zap.Error(logx.SafeError(err)))
		return errors.Join(cause, err)
	}
	return errors.Join(cause, sqlDB.Close())
}

// Run 启动 HTTP 服务并返回监听错误。
func (s *Server) Run(addr string) error {
	s.http.Addr = addr
	return s.http.ListenAndServe()
}

// RunTLS 由业务进程直接终止 TLS，无需额外反向代理进程。
func (s *Server) RunTLS(addr, cert, key string) error {
	s.http.Addr = addr
	return s.http.ListenAndServeTLS(cert, key)
}

// Stop 等待 HTTP 与清理协程退出；超时保留存储连接，允许调用方再次收尾。
func (s *Server) Stop(ctx context.Context) error {
	if s.drain != nil {
		s.drain.stop()
	}
	var cleanErr error
	if s.cleaner != nil {
		cleanErr = s.cleaner.stop(ctx)
	}
	err := s.http.Shutdown(ctx)
	var drainErr error
	if s.drain != nil {
		drainErr = s.drain.wait(ctx)
	}
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if err != nil || cleanErr != nil || drainErr != nil {
		zap.L().Error("server infrastructure failed", zap.Error(logx.SafeError(err)))
		return errors.Join(err, cleanErr, drainErr)
	}
	return s.closeClients()
}

// closeClients 汇总连接释放错误，前一项失败时仍尝试关闭其他依赖。
func (s *Server) closeClients() error {
	s.closeOnce.Do(func() { s.closeErr = s.releaseClients() })
	return s.closeErr
}

// releaseClients 仅在初始化失败或所有请求退出后释放外部连接。
func (s *Server) releaseClients() error {
	var err error
	if s.sql != nil {
		err = closeDB(s.sql, nil)
	}
	if s.redis != nil {
		err = errors.Join(err, s.redis.Close())
	}
	return err
}

// memoryService 根据服务器配置装配摘要与可选向量能力。
func memoryService(conn *gorm.DB, cfg config.Config, logger *zap.Logger) *memapp.Service {
	var embed memdomain.Embedder
	if cfg.EmbeddingURL != "" && cfg.EmbeddingModel != "" {
		embed = &meminfra.Embedder{URL: cfg.EmbeddingURL, Key: cfg.EmbeddingKey, ModelName: cfg.EmbeddingModel}
	}
	var summary memdomain.Summarizer
	var extractor memdomain.CandidateExtractor
	if cfg.ProviderURL != "" {
		provider := &providerinfra.OpenAI{URL: cfg.ProviderURL, Key: cfg.ProviderKey}
		summary = &meminfra.Summarizer{Provider: provider, Model: cfg.ProviderModel}
		extractor = &meminfra.Extractor{Provider: provider, Model: cfg.ProviderModel}
	}
	return memapp.New(meminfra.NewRepo(conn), msginfra.NewRepo(conn), logger,
		memapp.Options{Summary: summary, Embed: embed, Extract: extractor, Budget: cfg.ContextBudget})
}
