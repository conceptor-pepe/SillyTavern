// token.go 定义用户会话令牌的签发和解析规则。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrToken 表示会话令牌无效或已过期。
var ErrToken = errors.New("invalid token")

// Token 保存令牌中携带的用户身份。
type Token struct {
	UserID  uint64
	Version int
	Expire  time.Time
}

// Sign 使用 HMAC 签发短期用户令牌。
func Sign(secret string, userID uint64, version int, now time.Time) string {
	expire := now.Add(24 * time.Hour).Unix()
	body := strings.Join([]string{strconv.FormatUint(userID, 10), strconv.Itoa(version), strconv.FormatInt(expire, 10)}, ".")
	return body + "." + mac(secret, body)
}

// Parse 校验签名并返回令牌身份。
func Parse(secret, value string, now time.Time) (Token, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 || !validMac(secret, strings.Join(parts[:3], "."), parts[3]) {
		return Token{}, ErrToken
	}
	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return Token{}, ErrToken
	}
	version, err := strconv.Atoi(parts[1])
	if err != nil {
		return Token{}, ErrToken
	}
	expire, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || now.Unix() >= expire {
		return Token{}, ErrToken
	}
	return Token{UserID: userID, Version: version, Expire: time.Unix(expire, 0)}, nil
}

// mac 计算令牌签名。
func mac(secret, body string) string {
	sum := hmac.New(sha256.New, []byte(secret))
	_, _ = sum.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(sum.Sum(nil))
}

// validMac 使用恒定时间比较令牌签名。
func validMac(secret, body, value string) bool {
	want := mac(secret, body)
	return hmac.Equal([]byte(want), []byte(value))
}
