// file.go 读取二进制部署配置，不把密钥或配置内容写入错误信息。
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// LoadFile 在环境变量基础上覆盖 JSON 配置，拒绝拼写错误和多份 JSON。
func LoadFile(name string) (Config, error) {
	cfg := Load()
	if name == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return cfg, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, errors.New("invalid config JSON")
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return cfg, errors.New("unexpected trailing config")
	}
	return cfg, cfg.Check()
}

// Check 拒绝缺少存储、安全密钥及半套 TLS 配置的独立部署。
func (c Config) Check() error {
	if c.MySQLDSN == "" || len(c.AuthSecret) < 32 {
		return errors.New("database and strong auth secret required")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return errors.New("TLS certificate and key must be paired")
	}
	return nil
}
