// local.go 限定无 TLS 的本机开发模式，避免公网部署降级 Cookie。
package config

import "net"

// LocalHTTP 仅对明确绑定回环 IP 且未配置 TLS 的实例放行。
func (c Config) LocalHTTP() bool {
	if c.TLSCert != "" || c.TLSKey != "" {
		return false
	}
	host, _, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
