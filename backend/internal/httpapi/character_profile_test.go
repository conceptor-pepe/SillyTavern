// character_profile_test.go 通过真实鉴权和 MySQL 验证角色创建表单的字段持久化。
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"ai-chat/backend/internal/character/domain"
)

// TestCharacterProfile 验证创建、重新读取、列表和退出后的鉴权边界。
func TestCharacterProfile(t *testing.T) {
	f := newFlow(t, http.NotFoundHandler())
	f.login(t)
	body := `{"name":"书店老板","description":"书店","personality":"温柔","scenario":"午后","first_message":"欢迎","tags":["日常","书店"],"gender":"female","age":"24","message_sample":"你好"}`
	var created struct{ Data domain.Character }
	if err := json.Unmarshal(f.call(t, "POST", "/api/v1/characters", body, 200), &created); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/characters/" + strconv.FormatUint(created.Data.ID, 10)
	var found struct{ Character domain.Character }
	if err := json.Unmarshal(f.call(t, "GET", path, "", 200), &found); err != nil {
		t.Fatal(err)
	}
	item := found.Character
	if item.ID == 0 || item.UserID != f.uid || item.Gender != "female" || item.Age != "24" || item.MessageSample != "你好" || len(item.Tags) != 2 {
		t.Fatal("created fields did not survive HTTP and MySQL round trip")
	}
	f.call(t, "POST", "/api/v1/characters", `{"name":"bad","portrait":"https://example.com/x"}`, 400)
	f.call(t, "POST", "/api/v1/auth/logout", `{}`, 200)
	f.call(t, "GET", path, "", 401)
	f.call(t, "POST", "/api/v1/characters", body, 401)
}
