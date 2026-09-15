// password.go 提供用户密码的安全哈希和校验能力。
package app

import "golang.org/x/crypto/bcrypt"

// HashPass 使用 bcrypt 生成密码哈希。
func HashPass(password string) (string, error) {
	data, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MatchPass 校验明文密码和 bcrypt 哈希。
func MatchPass(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
