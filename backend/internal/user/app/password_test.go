// password_test.go 验证用户密码哈希不能反向还原且可正确校验。
package app

import "testing"

// TestPassword 验证密码哈希和错误密码校验。
func TestPassword(t *testing.T) {
	hash, err := HashPass("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !MatchPass("secret", hash) {
		t.Fatal("password should match")
	}
	if MatchPass("wrong", hash) {
		t.Fatal("wrong password should fail")
	}
}
