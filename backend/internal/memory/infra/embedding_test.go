// embedding_test.go 验证向量供应商协议与错误响应，不依赖外网。
package infra

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEmbeddingProtocol 校验乱序返回被还原且模型与文本按配置发送。
func TestEmbeddingProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string
			Input []string
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "embedding-test" || len(body.Input) != 2 {
			t.Error("wrong request")
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}]}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	embed := &Embedder{URL: server.URL, ModelName: "embedding-test"}
	rows, err := embed.Embed(t.Context(), []string{"a", "b"})
	if err != nil || len(rows) != 2 || rows[0][0] != 1 || rows[1][1] != 1 {
		t.Fatal("embedding order", err)
	}
}

// TestInvalidEmbedding 拒绝缺失索引、重复索引、零向量和供应商失败。
func TestInvalidEmbedding(t *testing.T) {
	for _, body := range []string{`{"data":[]}`, `{"data":[{"index":2,"embedding":[1]}]}`, `{"data":[{"index":0,"embedding":[0]}]}`, `not-json`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte(body)); err != nil {
				t.Error(err)
			}
		}))
		embed := &Embedder{URL: server.URL, ModelName: "test"}
		if _, err := embed.Embed(t.Context(), []string{"a"}); err == nil {
			t.Error("invalid embedding accepted")
		}
		server.Close()
	}
}
