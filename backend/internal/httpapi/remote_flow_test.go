// remote_flow_test.go 使用独立服务装配验证跨实例取消与超时清理会关闭原实例的上游连接。
package httpapi

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chatinfra "ai-chat/backend/internal/chat/infra"
	"ai-chat/backend/internal/config"
	genapp "ai-chat/backend/internal/generation/app"
	geninfra "ai-chat/backend/internal/generation/infra"
	"ai-chat/backend/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
)

// TestRemoteStop 验证控制请求落到另一个 Runner 时，原始 HTTP 流仍正确退出。
func TestRemoteStop(t *testing.T) {
	for _, action := range []string{"cancel", "expire"} {
		t.Run(action, func(t *testing.T) {
			ready, stopped := make(chan struct{}), make(chan struct{})
			f := newFlow(t, remoteUpstream(t, ready, stopped))
			f.login(t)
			f.seedChat(t)
			peer := peerFlow(t, f)
			response := f.open(t, "POST", "/api/v1/chats/"+f.chatID+"/generations",
				`{"model":"test","parent_id":"`+f.msgID+`"}`)
			reader := bufio.NewReader(response.Body)
			id := readStart(t, reader)
			awaitRemote(t, ready)
			peer.checkForeign(t, id)
			stopRemote(t, peer, id, action)
			remaining, err := io.ReadAll(reader)
			if err != nil || strings.Contains(string(remaining), "event:message_end") ||
				!strings.Contains(string(remaining), "event:generation_error") {
				t.Fatalf("remote stop response=%s err=%v", remaining, err)
			}
			awaitRemote(t, stopped)
			checkRemote(t, f, id, action)
		})
	}
}

// peerFlow 建立第二套生产路由、连接池和内存取消表，仅共享数据库及签名配置。
func peerFlow(t *testing.T, source *flowEnv) *flowEnv {
	t.Helper()
	server, err := New(config.Config{
		MySQLDSN: source.db.Dialector.(*mysql.Dialector).DSN, AuthSecret: "integration-test-secret",
		ProviderURL: "http://127.0.0.1:1",
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	api := httptest.NewTLSServer(server.http.Handler)
	t.Cleanup(api.Close)
	client := api.Client()
	client.Timeout = 10 * time.Second
	client.Jar = source.client.Jar
	return &flowEnv{url: api.URL, client: client, db: server.sql,
		uid: source.uid, chatID: source.chatID, msgID: source.msgID}
}

// remoteUpstream 保持真实网络连接到客户端取消，消费请求体以便检测对端断开。
func remoteUpstream(t *testing.T, ready, stopped chan struct{}) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(ready)
		select {
		case <-r.Context().Done():
			close(stopped)
		case <-time.After(8 * time.Second):
			t.Error("remote stop did not close upstream")
		}
	})
}

// awaitRemote 以事件同步测试，不使用固定睡眠假定模型已开始或退出。
func awaitRemote(t *testing.T, event <-chan struct{}) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(5 * time.Second):
		t.Fatal("remote lifecycle event timed out")
	}
}

// stopRemote 仅操作另一个实例的取消入口或生成域清理服务。
func stopRemote(t *testing.T, peer *flowEnv, id, action string) {
	t.Helper()
	if action == "cancel" {
		peer.call(t, "DELETE", "/api/v1/generations/"+id, "", http.StatusNoContent)
		return
	}
	tasks := genapp.New(geninfra.NewRepo(peer.db, chatinfra.NewGate(peer.db)))
	count, err := tasks.Expire(t.Context(), time.Now().Add(time.Hour).Unix())
	if err != nil || count != 1 {
		t.Fatalf("remote expiry count=%d err=%v", count, err)
	}
}

// checkRemote 确认原实例收尾不覆盖远端终态，也不留下部分 assistant 消息。
func checkRemote(t *testing.T, f *flowEnv, id, action string) {
	t.Helper()
	var task model.Generation
	if err := f.db.WithContext(t.Context()).First(&task, id).Error; err != nil {
		t.Fatal(err)
	}
	status, code := "cancelled", ""
	if action == "expire" {
		status, code = "failed", "generation_timeout"
	}
	if task.Status != status || task.ErrorCode != code || task.MessageID != nil || task.FinishedAt == nil {
		t.Fatalf("remote terminal state overwritten: %+v", task)
	}
	var count int64
	err := f.db.WithContext(t.Context()).Model(&model.Message{}).
		Where("conversation_id = ? AND role = ?", f.chatID, "assistant").Count(&count).Error
	if err != nil || count != 0 {
		t.Fatalf("partial message count=%d err=%v", count, err)
	}
	f.call(t, "DELETE", "/api/v1/generations/"+id, "", http.StatusNotFound)
}
