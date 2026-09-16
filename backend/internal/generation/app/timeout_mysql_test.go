// timeout_mysql_test.go 验证真实 HTTP 上游超时断开与 MySQL 失败补偿。
package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chatinfra "ai-chat/backend/internal/chat/infra"
	"ai-chat/backend/internal/generation/domain"
	geninfra "ai-chat/backend/internal/generation/infra"
	"ai-chat/backend/internal/model"
	providerinfra "ai-chat/backend/internal/provider/infra"
	"ai-chat/backend/internal/testdb"
	"gorm.io/gorm"
)

// TestTimeoutHTTP 覆盖响应头迟迟不返回，以及返回头后流式正文卡住两种网络状态。
func TestTimeoutHTTP(t *testing.T) {
	for _, headers := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[headers], func(t *testing.T) {
			db := timeoutDB(t)
			gate := chatinfra.NewGate(db)
			tasks := New(geninfra.NewRepo(db, gate))
			task, err := tasks.Create(t.Context(), domain.Generation{
				UserID: 7, ConversationID: 3, Provider: "test", Model: "test",
			})
			if err != nil {
				t.Fatal(err)
			}
			stopped := make(chan struct{})
			upstream := stalledHTTP(t, headers, stopped)
			runner := NewRunner(tasks, &providerinfra.OpenAI{URL: upstream.URL}, geninfra.NewDoneWriter(db, gate))
			runner.limit = 200 * time.Millisecond
			_, err = runner.Run(t.Context(), RunArgs{UserID: 7, ConversationID: 3, GenerationID: task.ID})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected deadline: %v", err)
			}
			select {
			case <-stopped:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream connection remained open")
			}
			checkTimeout(t, db, tasks, task.ID)
		})
	}
}

// stalledHTTP 在请求断开前保持阻塞，证明客户端确实终止底层网络请求。
func stalledHTTP(t *testing.T, headers bool, stopped chan struct{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 消费完整请求体，使服务端可以持续检测客户端断开，而非停留在未读正文。
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
			return
		}
		if headers {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
		}
		select {
		case <-r.Context().Done():
			close(stopped)
		case <-time.After(3 * time.Second):
			t.Error("upstream deadline not propagated")
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// timeoutDB 使用隔离真实库，仅建立任务完成事务所需的表。
func timeoutDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testdb.Open(t).WithContext(t.Context())
	if err := db.AutoMigrate(&model.Conversation{}, &model.Generation{}, &model.Message{}, &model.MessageVariant{}); err != nil {
		t.Fatal(err)
	}
	chat := model.Conversation{Base: model.Base{ID: 3}, UserID: 7, CharacterID: 1, ExtraData: "{}"}
	if err := db.Create(&chat).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

// checkTimeout 对照任务与消息表，确认超时已落库但没有部分回复。
func checkTimeout(t *testing.T, db *gorm.DB, tasks *Service, id uint64) {
	t.Helper()
	task, err := tasks.Find(t.Context(), 7, id)
	if err != nil || task.Status != domain.StatusFailed || task.ErrorCode != "generation_timeout" ||
		task.ErrorMessage != "generation timed out" || task.FinishedAt == nil {
		t.Fatalf("task=%+v err=%v", task, err)
	}
	var count int64
	if err := db.WithContext(t.Context()).Model(&model.Message{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("partial message count=%d err=%v", count, err)
	}
}
