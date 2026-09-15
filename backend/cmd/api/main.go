// main.go 负责加载配置、创建服务并处理进程退出。
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-chat/backend/internal/config"
	"ai-chat/backend/internal/httpapi"
	"ai-chat/backend/internal/logx"
	"go.uber.org/zap"
)

// main 启动 AI Chat API 服务。
func main() {
	cfg := config.Load()
	logger, err := logx.New()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	server, err := httpapi.New(cfg, logger)
	if err != nil {
		logger.Fatal("api init failed", zap.Error(err))
	}
	go serve(server, cfg, logger)
	waitExit()
	shutdown(server, logger)
}

// serve 监听 HTTP 请求并记录启动错误。
func serve(server *httpapi.Server, cfg config.Config, logger *zap.Logger) {
	err := server.Run(cfg.HTTPAddr)
	if err != nil && !errors.Is(err, httpapi.ErrClosed) {
		logger.Error("api start failed", zap.Error(err))
	}
}

// waitExit 等待系统退出信号，避免直接终止请求中的连接。
func waitExit() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

// shutdown 在退出前关闭 HTTP 服务和外部连接。
func shutdown(server *httpapi.Server, logger *zap.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		logger.Error("api stop failed", zap.Error(err))
	}
}
