// shutdown_flow_test.go 验证生产服务停止会中止模型请求并在关闭存储前持久化终态。
package httpapi

import (
	"bufio"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"ai-chat/backend/internal/model"
)

// TestShutdownGeneration 通过真实登录、TLS、MySQL 和上游连接验证停止生成完整链路。
func TestShutdownGeneration(t *testing.T) {
	ready, stopped := make(chan struct{}), make(chan struct{})
	f := newFlow(t, remoteUpstream(t, ready, stopped))
	f.login(t)
	f.seedChat(t)
	response := f.open(t, "POST", "/api/v1/chats/"+f.chatID+"/generations",
		`{"model":"test","parent_id":"`+f.msgID+`"}`)
	reader := bufio.NewReader(response.Body)
	id := readStart(t, reader)
	awaitRemote(t, ready)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := f.server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	awaitRemote(t, stopped)
	body, err := io.ReadAll(reader)
	if err != nil || strings.Contains(string(body), "event:message_end") {
		t.Fatalf("shutdown stream body=%s err=%v", body, err)
	}
	checkShutdownTask(t, f, id)
	pool, err := f.server.sql.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.PingContext(t.Context()); err == nil {
		t.Fatal("server storage still open after shutdown")
	}
	if err := f.server.Stop(ctx); err != nil {
		t.Fatalf("repeated shutdown failed: %v", err)
	}
}

// checkShutdownTask 使用独立观察连接验证状态已保存，不以日志或内存状态代替数据库事实。
func checkShutdownTask(t *testing.T, f *flowEnv, id string) {
	t.Helper()
	var task model.Generation
	if err := f.db.WithContext(t.Context()).First(&task, id).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != "failed" || task.ErrorCode != "generation_failed" ||
		task.FinishedAt == nil || task.MessageID != nil || task.UserID != f.uid {
		t.Fatalf("shutdown did not preserve task lifecycle: %+v", task)
	}
	var count int64
	err := f.db.WithContext(t.Context()).Model(&model.Message{}).
		Where("conversation_id = ? AND role = ?", f.chatID, "assistant").Count(&count).Error
	if err != nil || count != 0 {
		t.Fatalf("shutdown left partial assistant: count=%d err=%v", count, err)
	}
}
