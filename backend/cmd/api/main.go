// main.go 负责加载配置、创建服务并处理进程退出。
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-chat/backend/internal/config"
	"ai-chat/backend/internal/httpapi"
	"ai-chat/backend/internal/logx"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// main 启动 AI Chat API 服务。
func main() {
	name := flag.String("config", "", "JSON config file")
	flag.Parse()
	logger, err := logx.New()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	cfg, err := config.LoadFile(*name)
	if err != nil {
		logger.Fatal("config invalid", zap.Error(logx.SafeError(err)))
	}
	gin.SetMode(gin.ReleaseMode)
	redis.SetLogger(logx.NewRedis(logger))
	server, err := httpapi.New(cfg, logger)
	if err != nil {
		logger.Fatal("api init failed", zap.Error(logx.SafeError(err)))
	}
	done := make(chan error, 1)
	go func() { done <- serve(server, cfg) }()
	err = waitExit(done)
	shutdown(server, logger)
	if err != nil && !errors.Is(err, httpapi.ErrClosed) {
		logger.Fatal("api start failed", zap.Error(logx.SafeError(err)))
	}
}

// serve 监听 HTTP 请求并记录启动错误。
func serve(server *httpapi.Server, cfg config.Config) error {
	if cfg.TLSCert != "" {
		return server.RunTLS(cfg.HTTPAddr, cfg.TLSCert, cfg.TLSKey)
	}
	return server.Run(cfg.HTTPAddr)
}

// waitExit 等待系统退出信号，避免直接终止请求中的连接。
func waitExit(done <-chan error) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)
	select {
	case <-ch:
		return nil
	case err := <-done:
		return err
	}
}

// shutdown 在退出前关闭 HTTP 服务和外部连接。
func shutdown(server *httpapi.Server, logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		logger.Error("api stop failed", zap.Error(logx.SafeError(err)))
		return
	}
	logger.Info("api stopped")
}
