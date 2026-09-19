// background_test.go 验证图片入口在无数据库环境也能拒绝远程资源与畸形图片。
package preferencehttp

import (
	"ai-chat/backend/internal/preference/domain"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"image"
	"image/jpeg"
	"net/http/httptest"
	"testing"
)

type backgroundFake struct {
	uid    uint64
	item   domain.Background
	writes int
}

// Get 返回模拟账号背景。
func (f *backgroundFake) Get(_ context.Context, uid uint64) (domain.Background, error) {
	f.uid = uid
	return f.item, nil
}

// Save 记录经校验后的账号编号与配置。
func (f *backgroundFake) Save(_ context.Context, uid uint64, item domain.Background) error {
	f.uid = uid
	f.item = item
	f.writes++
	return nil
}

// TestBackgroundInput 归属只能来自鉴权，畸形输入不会进入持久化。
func TestBackgroundInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &backgroundFake{}
	engine := gin.New()
	New(repo, zap.NewNop()).Routes(engine, func(c *gin.Context) { c.Set("user_id", uint64(7)) })
	for _, body := range []string{`{"image":"https://example.com/x","enabled":true}`, `{"image":"data:image/jpeg;base64,AAAA"}`, `invalid`} {
		response := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/v1/me/background", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(response, req)
		if response.Code != 400 || repo.writes != 0 {
			t.Fatal("invalid image persisted")
		}
	}
	var data bytes.Buffer
	if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(struct {
		Image   string `json:"image"`
		Enabled bool   `json:"enabled"`
		UserID  int    `json:"user_id"`
	}{"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data.Bytes()), true, 99})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/v1/me/background", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(response, req)
	if response.Code != 200 || repo.writes != 1 || repo.uid != 7 || !repo.item.Enabled {
		t.Fatal("valid background failed", response.Code)
	}
}
