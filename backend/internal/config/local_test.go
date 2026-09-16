// local_test.go 验证本地 Cookie 例外不能扩展到公网监听。
package config

import "testing"

// TestLocalHTTP 覆盖回环、公网、通配地址及 TLS 边界。
func TestLocalHTTP(t *testing.T) {
	for _, tc := range []struct {
		addr string
		cert string
		key  string
		want bool
	}{
		{"127.0.0.1:8080", "", "", true},
		{"[::1]:8080", "", "", true},
		{":8080", "", "", false},
		{"0.0.0.0:8080", "", "", false},
		{"192.168.1.1:8080", "", "", false},
		{"localhost:8080", "", "", false},
		{"invalid", "", "", false},
		{"127.0.0.1:8080", "cert", "key", false},
		{"127.0.0.1:8080", "", "key", false},
	} {
		cfg := Config{HTTPAddr: tc.addr, TLSCert: tc.cert, TLSKey: tc.key}
		if got := cfg.LocalHTTP(); got != tc.want {
			t.Errorf("addr=%s local=%v want=%v", tc.addr, got, tc.want)
		}
	}
}
