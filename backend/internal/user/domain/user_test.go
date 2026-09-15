// user_test.go 验证用户登录的领域边界。
package domain

import "testing"

// TestCheckLogin 验证启用状态和密码匹配规则。
func TestCheckLogin(t *testing.T) {
	user := User{Handle: "demo", Enabled: true, PasswordHash: "hash"}
	if err := user.CheckLogin("pass", func(got, want string) bool { return got == "pass" && want == "hash" }); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	disabled := user
	disabled.Enabled = false
	if err := disabled.CheckLogin("pass", func(string, string) bool { return true }); err != ErrDisabled {
		t.Fatalf("got %v, want %v", err, ErrDisabled)
	}
}
