// remove_flow_test.go 验证跨实例删除会话会取消生成、关闭上游并隐藏任务。
package httpapi

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"ai-chat/backend/internal/model"
)

// TestRemoteRemove 使用真实 TLS、Cookie、MySQL 和独立服务装配验证删除链路。
func TestRemoteRemove(t *testing.T) {
	ready, stopped := make(chan struct{}), make(chan struct{})
	f := newFlow(t, remoteUpstream(t, ready, stopped))
	f.login(t)
	f.seedChat(t)
	peer := peerFlow(t, f)
	path := "/api/v1/chats/" + f.chatID
	f.call(t, "PUT", path+"/favorite", "", 200)
	response := f.open(t, "POST", path+"/generations",
		`{"model":"test","parent_id":"`+f.msgID+`"}`)
	reader := bufio.NewReader(response.Body)
	id := readStart(t, reader)
	awaitRemote(t, ready)
	peer.call(t, "DELETE", path, "", 200)
	remaining, err := io.ReadAll(reader)
	if err != nil || strings.Contains(string(remaining), "event:message_end") ||
		!strings.Contains(string(remaining), "event:generation_error") {
		t.Fatalf("delete response=%s err=%v", remaining, err)
	}
	awaitRemote(t, stopped)
	checkRemote(t, f, id, "cancel")
	f.call(t, "GET", "/api/v1/generations/"+id, "", 404)
	f.call(t, "GET", path, "", 404)
	f.call(t, "DELETE", path, "", 200)
	f.call(t, "PUT", path+"/favorite", "", 404)
	f.call(t, "POST", path+"/generations", `{"model":"test"}`, 404)
	var count int64
	err = f.db.Model(&model.Favorite{}).
		Where("user_id = ? AND character_id = ? AND kind = ?", f.uid, f.chatID, "chat").Count(&count).Error
	if err != nil || count != 0 {
		t.Fatalf("orphan favorites=%d err=%v", count, err)
	}
}
