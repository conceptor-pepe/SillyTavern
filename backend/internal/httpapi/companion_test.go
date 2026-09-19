// companion_test.go 使用真实 MySQL 与 HTTP 验证角色资料、长期记忆进入实际 Provider 请求。
package httpapi

import (
	character "ai-chat/backend/internal/character/domain"
	provider "ai-chat/backend/internal/provider/domain"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// TestCompanionContextMySQL 覆盖编辑、世界书触发、记忆注入、遗忘与角色删除后的历史保留。
func TestCompanionContextMySQL(t *testing.T) {
	requests := make(chan provider.Request, 4)
	f := newFlow(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body provider.Request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests <- body
		w.Header().Set("Content-Type", "text/event-stream")
		_, writeErr := w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"你好\"}}]}\n\ndata: [DONE]\n\n"))
		if writeErr != nil {
			t.Error(writeErr)
		}
	}))
	f.login(t)
	f.seedChat(t)
	var result struct{ Items []character.Character }
	if err := json.Unmarshal(f.call(t, "GET", "/api/v1/characters", "", 200), &result); err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatUint(result.Items[0].ID, 10)
	path := "/api/v1/characters/" + id
	f.call(t, "PUT", path, `{"name":"书店","gender":"female","age":"24","message_sample":"梦里见","scenario":"月港"}`, 200)
	memory := responseID(t, f.call(t, "POST", path+"/memories", `{"kind":"fact","content":"用户的昵称是小鹿","pinned":true,"enabled":true}`, 200))
	f.call(t, "POST", path+"/memories", `{"kind":"lore","content":"月港在北方","keywords":["你好"],"enabled":true}`, 200)
	f.call(t, "POST", "/api/v1/chats/"+f.chatID+"/generations", `{"model":"test","parent_id":"`+f.msgID+`"}`, 200)
	req := <-requests
	all := ""
	for _, m := range req.Messages {
		all += m.Content
	}
	for _, value := range []string{"年龄：24", "梦里见", "小鹿", "月港在北方"} {
		if !strings.Contains(all, value) {
			t.Fatal("context missing", value)
		}
	}
	f.call(t, "DELETE", path+"/memories/"+memory, "", 200)
	f.call(t, "POST", "/api/v1/chats/"+f.chatID+"/generations", `{"model":"test","parent_id":"`+f.msgID+`"}`, 200)
	req = <-requests
	for _, m := range req.Messages {
		if strings.Contains(m.Content, "小鹿") {
			t.Fatal("deleted memory retrieved")
		}
	}
	f.call(t, "POST", "/api/v1/chats/"+f.chatID+"/generations", `{"model":"test","content":"legacy"}`, 400)
	f.call(t, "DELETE", path, "", 200)
	f.call(t, "GET", path, "", 404)
	f.call(t, "GET", "/api/v1/chats/"+f.chatID+"/messages", "", 200)
	f.call(t, "POST", "/api/v1/chats/"+f.chatID+"/generations", `{"model":"test","parent_id":"`+f.msgID+`"}`, 404)
}
