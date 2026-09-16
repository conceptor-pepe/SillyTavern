// profile.go 校验角色展示资料并重新编码封面，避免保存原始图片元数据。
package app

import (
	"bytes"
	"encoding/base64"
	"image/jpeg"
	"strings"

	"ai-chat/backend/internal/character/domain"
)

// cleanProfile 限制展示字段和文本大小，保留既有角色创建兼容性。
func cleanProfile(item *domain.Character) error {
	if len(item.Tags) > 9 || len([]rune(item.Age)) > 8 {
		return domain.ErrInvalid
	}
	if item.Gender != "" && item.Gender != "female" && item.Gender != "male" && item.Gender != "other" {
		return domain.ErrInvalid
	}
	for _, value := range []string{item.Description, item.Personality, item.Scenario, item.FirstMessage, item.MessageSample} {
		if len(value) > 60000 {
			return domain.ErrInvalid
		}
	}
	tags := make([]string, 0, len(item.Tags))
	seen := make(map[string]bool)
	for _, tag := range item.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len([]rune(tag)) > 24 || seen[tag] {
			return domain.ErrInvalid
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	item.Tags = tags
	portrait, err := cleanPortrait(item.Portrait)
	if err != nil {
		return err
	}
	item.Portrait = portrait
	return nil
}

// cleanPortrait 只接收有界 JPEG 缩略图，重编码后清除 EXIF 和角色卡内嵌文本。
func cleanPortrait(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	const prefix = "data:image/jpeg;base64,"
	if !strings.HasPrefix(value, prefix) || len(value) > 350000 {
		return "", domain.ErrInvalid
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		return "", domain.ErrInvalid
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width > 1024 || cfg.Height > 1024 {
		return "", domain.ErrInvalid
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return "", domain.ErrInvalid
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", domain.ErrInvalid
	}
	return prefix + base64.StdEncoding.EncodeToString(out.Bytes()), nil
}
