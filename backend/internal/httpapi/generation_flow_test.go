// generation_flow_test.go 验证真实 HTTP、MySQL 和模拟模型上游之间的生成与取消闭环。
package httpapi

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"ai-chat/backend/internal/model"
	provider "ai-chat/backend/internal/provider/domain"
)

// TestGenerationFlow 验证登录、消息持久化、多候选事件与刷新读取一致。
func TestGenerationFlow(t *testing.T) {
	f := newFlow(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request provider.Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if request.N != 2 || len(request.Messages) == 0 ||
			request.Messages[len(request.Messages)-1].Content != "你好" {
			t.Errorf("unexpected prompt: %+v", request)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, err := io.WriteString(w, "data: "+`{"choices":[{"index":0,"delta":{"content":"回答甲"}},{"index":1,"delta":{"content":"回答乙"}}]}`+"\n\ndata: [DONE]\n\n")
		if err != nil {
			t.Error(err)
		}
	}))
	f.login(t)
	f.seedChat(t)
	body := `{"model":"test","n":2,"parent_id":"` + f.msgID + `"}`
	events := string(f.call(t, "POST", "/api/v1/chats/"+f.chatID+"/generations", body, 200))
	for _, event := range []string{"event:message_start", "event:message_delta", `"index":1`, "event:message_end"} {
		if !strings.Contains(events, event) {
			t.Fatalf("missing %s: %s", event, events)
		}
	}
	f.checkDone(t)
	f.call(t, "POST", "/api/v1/auth/logout", `{}`, 200)
	f.call(t, "GET", "/api/v1/chats/"+f.chatID+"/messages", "", 401)
}

// checkDone 对照数据库事实验证任务完成、父节点保留和候选可见。
func (f *flowEnv) checkDone(t *testing.T) {
	t.Helper()
	var task model.Generation
	if err := f.db.WithContext(t.Context()).Where("conversation_id = ?", f.chatID).First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != "completed" || task.MessageID == nil || task.FinishedAt == nil {
		t.Fatalf("task=%+v", task)
	}
	var msg model.Message
	if err := f.db.WithContext(t.Context()).First(&msg, *task.MessageID).Error; err != nil {
		t.Fatal(err)
	}
	if msg.ParentID == nil || strconv.FormatUint(*msg.ParentID, 10) != f.msgID || msg.Content != "回答甲" {
		t.Fatalf("message=%+v", msg)
	}
	messages := string(f.call(t, "GET", "/api/v1/chats/"+f.chatID+"/messages", "", 200))
	if !strings.Contains(messages, "回答甲") || !strings.Contains(messages, "你好") {
		t.Fatalf("refresh=%s", messages)
	}
	id := strconv.FormatUint(msg.ID, 10)
	value := f.call(t, "GET", "/api/v1/messages/"+id+"/variants", "", 200)
	var result struct {
		Data struct{ Items []struct{ ID string } }
	}
	if err := json.Unmarshal(value, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data.Items) != 2 {
		t.Fatalf("variants=%s", value)
	}
	path := "/api/v1/messages/" + id + "/variants/" + result.Data.Items[1].ID + "/select"
	first := responseID(t, f.call(t, "POST", path, `{}`, 200))
	second := responseID(t, f.call(t, "POST", path, `{}`, 200))
	if first != second || first == id {
		t.Fatalf("selection must create one separate branch: %s %s %s", id, first, second)
	}
}

// TestCancelFlow 验证取消接口中断上游网络请求且不落库不完整回复。
func TestCancelFlow(t *testing.T) {
	ready, stopped := make(chan struct{}), make(chan struct{})
	f := newFlow(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		close(ready)
		select {
		case <-r.Context().Done():
			close(stopped)
		case <-time.After(5 * time.Second):
			t.Error("upstream was not cancelled")
		}
	}))
	f.login(t)
	f.seedChat(t)
	response := f.open(t, "POST", "/api/v1/chats/"+f.chatID+"/generations",
		`{"model":"test","parent_id":"`+f.msgID+`"}`)
	reader := bufio.NewReader(response.Body)
	id := readStart(t, reader)
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("upstream did not start")
	}
	f.checkForeign(t, id)
	f.call(t, "DELETE", "/api/v1/generations/"+id, "", 204)
	remaining, err := io.ReadAll(reader)
	if err != nil || strings.Contains(string(remaining), "event:message_end") {
		t.Fatalf("cancelled stream: %s err=%v", remaining, err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("provider connection remained open")
	}
	f.checkCancelled(t, id)
}

// readStart 从真实流响应中读取任务编号，不使用伪造令牌或数据库查询代替接口。
func readStart(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("missing start event: %v", err)
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var start struct {
			ID uint64 `json:"generation_id"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), &start); err != nil {
			t.Fatal(err)
		}
		if start.ID == 0 {
			t.Fatal("missing generation id")
		}
		return strconv.FormatUint(start.ID, 10)
	}
}

// checkCancelled 校验取消终态没有被 Runner 失败收尾覆盖，重复取消无副作用。
func (f *flowEnv) checkCancelled(t *testing.T, id string) {
	t.Helper()
	var task model.Generation
	if err := f.db.WithContext(t.Context()).First(&task, id).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != "cancelled" || task.MessageID != nil || task.FinishedAt == nil {
		t.Fatalf("task=%+v", task)
	}
	var count int64
	err := f.db.WithContext(t.Context()).Model(&model.Message{}).
		Where("conversation_id = ? AND role = ?", f.chatID, "assistant").Count(&count).Error
	if err != nil || count != 0 {
		t.Fatalf("partial assistant count=%d err=%v", count, err)
	}
	f.call(t, "DELETE", "/api/v1/generations/"+id, "", 404)
}
