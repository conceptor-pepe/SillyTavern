// handler.go 提供只读嵌入资源，拒绝目录枚举和未知 API 的页面回退。
package webui

import (
	"bytes"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// Handler 创建前端处理器；没有嵌入资源时所有请求返回 404。
func Handler() http.Handler { return handler(files()) }

// handler 只允许 GET/HEAD 精确文件读取，不访问服务器磁盘。
func handler(root fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if root == nil || !fs.ValidPath(name) || (r.Method != "GET" && r.Method != "HEAD") {
			http.NotFound(w, r)
			return
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}
