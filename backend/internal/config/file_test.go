// file_test.go 验证独立部署配置覆盖、敏感错误及无效配置拒绝。
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigFile 验证文件覆盖环境变量，并保留未指定的环境配置。
func TestConfigFile(t *testing.T) {
	t.Setenv("AI_CHAT_ADDR", ":9000")
	t.Setenv("AI_CHAT_REDIS_ADDR", "redis:6379")
	name := writeConfig(t, `{"HTTPAddr":"127.0.0.1:8443","MySQLDSN":"test",
		"AuthSecret":"01234567890123456789012345678901"}`)
	cfg, err := LoadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8443" || cfg.RedisAddr != "redis:6379" {
		t.Fatal("config precedence changed")
	}
}

// TestConfigReject 拒绝未知字段、多份配置、弱密钥与不完整 TLS。
func TestConfigReject(t *testing.T) {
	for _, body := range []string{
		`{"secret-value":"private"}`, `{} {}`, `{`, `{}`,
		`{"MySQLDSN":"test","AuthSecret":"01234567890123456789012345678901","TLSCert":"cert"}`,
	} {
		_, err := LoadFile(writeConfig(t, body))
		if err == nil || strings.Contains(err.Error(), "private") {
			t.Fatal("invalid or sensitive config accepted")
		}
	}
}

// writeConfig 将合成配置限制在独立测试目录，不读取开发者凭据。
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(name, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}
