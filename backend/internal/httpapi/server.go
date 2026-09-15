// server.go 负责创建 Gin 服务、基础中间件和健康检查。
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	charhttp "ai-chat/backend/internal/character/http"
	charinfra "ai-chat/backend/internal/character/infra"
	chathttp "ai-chat/backend/internal/chat/http"
	chatinfra "ai-chat/backend/internal/chat/infra"
	"ai-chat/backend/internal/config"
	"ai-chat/backend/internal/db"
	genapp "ai-chat/backend/internal/generation/app"
	gendomain "ai-chat/backend/internal/generation/domain"
	genhttp "ai-chat/backend/internal/generation/http"
	geninfra "ai-chat/backend/internal/generation/infra"
	msghttp "ai-chat/backend/internal/message/http"
	msginfra "ai-chat/backend/internal/message/infra"
	providerinfra "ai-chat/backend/internal/provider/infra"
	userhttp "ai-chat/backend/internal/user/http"
	userinfra "ai-chat/backend/internal/user/infra"
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
	stopClean context.CancelFunc
}

// New 创建只包含基础能力的 API 服务。
func New(cfg config.Config, logger *zap.Logger) (*Server, error) {
	engine := gin.New()
	engine.Use(gin.Recovery(), requestID())
	registerHealth(engine)
	server := &Server{
		http: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
	if cfg.RedisAddr != "" {
		client, err := db.OpenRedis(context.Background(), cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
		if err != nil {
			return nil, err
		}
		server.redis = client
		logger.Info("redis ready", zap.String("addr", cfg.RedisAddr))
	}
	if cfg.MySQLDSN != "" {
		conn, err := openDB(cfg)
		if err != nil {
			return nil, err
		}
		server.sql = conn
		registerModules(engine, conn, cfg, logger)
		if cfg.ProviderURL != "" {
			tasks := genapp.New(geninfra.NewRepo(conn))
			provider := &providerinfra.OpenAI{URL: cfg.ProviderURL, Key: cfg.ProviderKey}
			runner := genapp.NewRunner(tasks, provider, msginfra.NewRepo(conn))
			genhttp.New(tasks, runner, genhttp.Deps{
				Chats: chatinfra.NewRepo(conn), Chars: charinfra.NewRepo(conn), Msgs: msginfra.NewRepo(conn),
			}, logger).Routes(engine, RequireAuth(cfg.AuthSecret))
			cleanCtx, stop := context.WithCancel(context.Background())
			server.stopClean = stop
			go cleanTasks(cleanCtx, tasks, logger)
			logger.Info("provider ready", zap.String("model", cfg.ProviderModel),
				zap.String("provider", "openai"))
		} else {
			logger.Info("provider disabled", zap.String("reason", "AI_CHAT_PROVIDER_URL is empty"),
				zap.String("status", gendomain.StatusPending))
		}
		logger.Info("mysql ready")
	}
	return server, nil
}

// registerModules 注册数据库相关的业务路由。
func registerModules(engine *gin.Engine, conn *gorm.DB, cfg config.Config, logger *zap.Logger) {
	auth := RequireAuth(cfg.AuthSecret)
	userhttp.NewLogin(userinfra.NewRepo(conn), cfg.AuthSecret, logger).RegisterRoutes(engine, auth)
	charhttp.New(charinfra.NewRepo(conn), logger).Routes(engine, auth)
	chats := chatinfra.NewRepo(conn)
	chathttp.New(chats, charinfra.NewRepo(conn), logger, chats).Routes(engine, auth)
	msghttp.New(msginfra.NewRepo(conn), chatinfra.NewRepo(conn), logger).Routes(engine, auth)
}

// openDB 创建 MySQL 连接并执行结构迁移。
func openDB(cfg config.Config) (*gorm.DB, error) {
	conn, err := db.OpenMySQL(context.Background(), cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(context.Background(), conn); err != nil {
		return nil, closeDB(conn, err)
	}
	return conn, nil
}

// closeDB 关闭迁移失败时的数据库连接，并保留原始错误。
func closeDB(conn *gorm.DB, cause error) error {
	sqlDB, err := conn.DB()
	if err != nil {
		return errors.Join(cause, err)
	}
	return errors.Join(cause, sqlDB.Close())
}

// Run 启动 HTTP 服务并返回监听错误。
func (s *Server) Run(addr string) error {
	s.http.Addr = addr
	return s.http.ListenAndServe()
}

// Stop 在超时时间内关闭 HTTP 服务。
func (s *Server) Stop(ctx context.Context) error {
	if s.stopClean != nil {
		s.stopClean()
	}
	err := s.http.Shutdown(ctx)
	if s.sql != nil {
		sqlDB, closeErr := s.sql.DB()
		if closeErr != nil {
			return closeErr
		}
		if closeErr := sqlDB.Close(); closeErr != nil {
			return closeErr
		}
	}
	if s.redis != nil {
		if closeErr := s.redis.Close(); closeErr != nil {
			return closeErr
		}
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// cleanTasks 周期清理超过十分钟仍运行的生成任务。
func cleanTasks(ctx context.Context, tasks *genapp.Service, logger *zap.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			count, err := tasks.Expire(ctx, time.Now().Add(-10*time.Minute).Unix())
			if err != nil {
				logger.Warn("generation cleanup failed", zap.Error(err))
				continue
			}
			if count > 0 {
				logger.Info("generation cleanup done", zap.Int64("count", count))
			}
		case <-ctx.Done():
			return
		}
	}
}
