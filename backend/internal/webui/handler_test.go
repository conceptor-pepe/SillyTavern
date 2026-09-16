// handler_test.go 验证静态资源边界、文件类型与 HEAD，不使用运行时磁盘。
package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// TestStaticFiles 验证首页和模块可用，目录、秘密、未知接口不可见。
func TestStaticFiles(t *testing.T) {
	root := fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte("<html>chat</html>")},
		"scripts/main.js": &fstest.MapFile{Data: []byte("export const ready=true;")},
	}
	for _, item := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/", 200}, {"HEAD", "/", 200},
		{"GET", "/scripts/main.js", 200}, {"GET", "/scripts", 404},
		{"POST", "/", 404}, {"GET", "/api/v1/missing", 404},
		{"GET", "/.env", 404}, {"GET", "/../index.html", 404},
	} {
		w := httptest.NewRecorder()
		handler(root).ServeHTTP(w, httptest.NewRequest(item.method, item.path, nil))
		if w.Code != item.status {
			t.Fatalf("%s %s: %d", item.method, item.path, w.Code)
		}
		if item.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD returned body")
		}
	}
	w := httptest.NewRecorder()
	handler(root).ServeHTTP(w, httptest.NewRequest("GET", "/scripts/main.js", nil))
	if !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
		t.Fatal("module MIME missing")
	}
}

// TestStaticDisabled 确认纯 API 构建不会意外暴露当前工作目录。
func TestStaticDisabled(t *testing.T) {
	w := httptest.NewRecorder()
	handler(nil).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 404 {
		t.Fatal("missing bundle should return 404")
	}
}
