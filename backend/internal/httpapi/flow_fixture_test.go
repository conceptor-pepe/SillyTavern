// flow_fixture_test.go 使用隔离 MySQL 和生产路由装配提供真实 HTTP 测试环境。
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"ai-chat/backend/internal/config"
	"ai-chat/backend/internal/model"
	"ai-chat/backend/internal/testdb"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// flowEnv 保存真实 HTTP 客户端和仅用于断言的数据库连接。
type flowEnv struct {
	server *Server
	url    string
	client *http.Client
	db     *gorm.DB
	uid    uint64
	chatID string
	msgID  string
}

// newFlow 启动生产装配的路由，TLS 用于验证 Secure Cookie 的实际传输。
func newFlow(t *testing.T, upstream http.Handler) *flowEnv {
	t.Helper()
	conn := testdb.Open(t)
	provider := httptest.NewServer(upstream)
	t.Cleanup(provider.Close)
	server, err := New(config.Config{
		MySQLDSN: conn.Dialector.(*mysql.Dialector).DSN, AuthSecret: "integration-test-secret",
		ProviderURL: provider.URL, ProviderModel: "test", RedisAddr: os.Getenv("AI_CHAT_TEST_REDIS_ADDR"),
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := server.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	api := httptest.NewUnstartedServer(server.http.Handler)
	api.Config = server.http
	api.StartTLS()
	t.Cleanup(api.Close)
	client := api.Client()
	client.Timeout = 10 * time.Second
	client.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &flowEnv{url: api.URL, client: client, db: conn, server: server}
}

// open 发起实际网络请求，流式响应由调用方消费并关闭。
func (f *flowEnv) open(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, f.url+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := f.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	})
	return response
}

// call 验证状态码并读取完整响应，禁止失败请求被误当成测试成功。
func (f *flowEnv) call(t *testing.T, method, path, body string, status int) []byte {
	t.Helper()
	response := f.open(t, method, path, body)
	value, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("%s %s: status=%d body=%s", method, path, response.StatusCode, value)
	}
	return value
}

// login 覆盖匿名拒绝、注册、退出、密码错误和正确登录的完整 Cookie 链路。
func (f *flowEnv) login(t *testing.T) {
	t.Helper()
	const account = `{"handle":"http-flow","password":"local-test-password","name":"集成测试"}`
	f.call(t, "GET", "/api/v1/me", "", 401)
	value := f.call(t, "POST", "/api/v1/auth/register", account, 201)
	var registered struct{ User struct{ ID uint64 } }
	if err := json.Unmarshal(value, &registered); err != nil {
		t.Fatal(err)
	}
	f.uid = registered.User.ID
	f.call(t, "POST", "/api/v1/auth/logout", `{}`, 200)
	f.call(t, "GET", "/api/v1/me", "", 401)
	f.call(t, "POST", "/api/v1/auth/login", `{"handle":"http-flow","password":"wrong"}`, 401)
	f.call(t, "POST", "/api/v1/auth/login", account, 200)
	f.call(t, "GET", "/api/v1/me", "", 200)
}

// seedChat 只通过数据库准备角色，会话和用户消息必须经生产接口创建。
func (f *flowEnv) seedChat(t *testing.T) {
	t.Helper()
	role := model.Character{UserID: f.uid, Name: "测试角色", Tags: "[]", ExtraData: "{}"}
	if err := f.db.WithContext(t.Context()).Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	body := `{"character_id":` + strconv.FormatUint(role.ID, 10) + `,"title":"HTTP test"}`
	f.chatID = responseID(t, f.call(t, "POST", "/api/v1/chats", body, 200))
	f.msgID = responseID(t, f.call(t, "POST", "/api/v1/chats/"+f.chatID+"/messages",
		`{"content":"你好","parent_id":null}`, 200))
}

// responseID 校验业务接口使用字符串 ID，避免前端整数精度丢失。
func responseID(t *testing.T, value []byte) string {
	t.Helper()
	var result struct{ Data struct{ ID string } }
	if err := json.Unmarshal(value, &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.ID == "" {
		t.Fatalf("missing data.id: %s", value)
	}
	return result.Data.ID
}

// checkForeign 使用另一个真实注册身份验证跨用户查询和取消不能影响任务。
func (f *flowEnv) checkForeign(t *testing.T, taskID string) {
	t.Helper()
	client := *f.client
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client.Jar = jar
	other := &flowEnv{url: f.url, client: &client}
	other.call(t, "POST", "/api/v1/auth/register",
		`{"handle":"other-user","password":"local-other-password"}`, 201)
	other.call(t, "GET", "/api/v1/chats/"+f.chatID, "", 404)
	other.call(t, "GET", "/api/v1/generations/"+taskID, "", 404)
	other.call(t, "DELETE", "/api/v1/generations/"+taskID, "", 404)
}
