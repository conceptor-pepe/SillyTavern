// config.go 负责读取 AI Chat 后端运行配置。
package config

import "os"

// Config 保存 HTTP、MySQL 和 Redis 的连接配置。
type Config struct {
	HTTPAddr      string
	MySQLDSN      string
	RedisAddr     string
	RedisPass     string
	RedisDB       int
	AuthSecret    string
	ProviderURL   string
	ProviderKey   string
	ProviderModel string
	TLSCert       string
	TLSKey        string
}

// Load 读取环境变量并填充本地开发默认值。
func Load() Config {
	return Config{
		HTTPAddr:      env("AI_CHAT_ADDR", ":8080"),
		MySQLDSN:      os.Getenv("AI_CHAT_MYSQL_DSN"),
		RedisAddr:     os.Getenv("AI_CHAT_REDIS_ADDR"),
		RedisPass:     os.Getenv("AI_CHAT_REDIS_PASS"),
		RedisDB:       0,
		AuthSecret:    env("AI_CHAT_AUTH_SECRET", "dev-only-secret"),
		ProviderURL:   os.Getenv("AI_CHAT_PROVIDER_URL"),
		ProviderKey:   os.Getenv("AI_CHAT_PROVIDER_KEY"),
		ProviderModel: env("AI_CHAT_PROVIDER_MODEL", "gpt-4o-mini"),
		TLSCert:       os.Getenv("AI_CHAT_TLS_CERT"),
		TLSKey:        os.Getenv("AI_CHAT_TLS_KEY"),
	}
}

// env 返回环境变量值，不存在时使用默认值。
func env(key, fallback string) string {
	value := os.Getenv(key)
	if value != "" {
		return value
	}
	return fallback
}
