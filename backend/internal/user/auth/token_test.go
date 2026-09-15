// token_test.go 验证会话令牌的签发、解析和过期规则。
package auth

import (
	"testing"
	"time"
)

// TestToken 验证有效令牌和篡改令牌都会得到正确结果。
func TestToken(t *testing.T) {
	now := time.Unix(1000, 0)
	value := Sign("secret", 7, 2, now)
	token, err := Parse("secret", value, now)
	if err != nil || token.UserID != 7 || token.Version != 2 {
		t.Fatalf("parse token: %#v, %v", token, err)
	}
	if _, err := Parse("bad", value, now); err != ErrToken {
		t.Fatalf("want invalid token, got %v", err)
	}
}
