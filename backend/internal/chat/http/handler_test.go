// handler_test.go 验证会话 HTTP 参数边界。
package chathttp

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestPageArgs 验证分页参数的合法范围。
func TestPageArgs(t *testing.T) {
	tests := []struct {
		query string
		ok    bool
	}{
		{"", true},
		{"?page=1&size=100", true},
		{"?page=0", false},
		{"?page=1000001", false},
		{"?size=0", false},
		{"?size=101", false},
	}
	for _, test := range tests {
		c := httptest.NewRequest("GET", "/api/v1/chats"+test.query, nil)
		query, _ := gin.CreateTestContext(httptest.NewRecorder())
		query.Request = c
		_, _, err := pageArgs(query)
		if (err == nil) != test.ok {
			t.Fatalf("query=%q err=%v", test.query, err)
		}
	}
}
