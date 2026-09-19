// profile.go 校验角色展示资料并重新编码封面，避免保存原始图片元数据。
package app

import (
	"bytes"
	"encoding/base64"
	"image/jpeg"
	"strings"

	"ai-chat/backend/internal/character/domain"
	"go.uber.org/zap"
)

// cleanProfile 限制展示字段和文本大小，保留既有角色创建兼容性。
func cleanProfile(item *domain.Character) error {
	if len(item.Tags) > 9 || len([]rune(item.Age)) > 8 {
		zap.L().Warn("character profile rejected", zap.String("reason", "profile limits"))
		return domain.ErrInvalid
	}
	if item.Gender != "" && item.Gender != "female" && item.Gender != "male" && item.Gender != "other" {
		zap.L().Warn("character profile rejected", zap.String("reason", "gender"))
		return domain.ErrInvalid
	}
	for _, value := range []string{item.Description, item.Personality, item.Scenario, item.FirstMessage, item.MessageSample} {
		if len(value) > 60000 {
			zap.L().Warn("character profile rejected", zap.String("reason", "text length"))
			return domain.ErrInvalid
		}
	}
	tags := make([]string, 0, len(item.Tags))
	seen := make(map[string]bool)
	for _, tag := range item.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len([]rune(tag)) > 24 || seen[tag] {
			zap.L().Warn("character profile rejected", zap.String("reason", "tag"))
			return domain.ErrInvalid
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	item.Tags = tags
	portrait, err := CleanPortrait(item.Portrait)
	if err != nil {
		zap.L().Warn("character profile rejected", zap.String("reason", "portrait"), zap.Error(err))
		return err
	}
	item.Portrait = portrait
	return nil
}

// CleanPortrait 只接收有界 JPEG 缩略图，重编码后清除 EXIF 和角色卡内嵌文本。
func CleanPortrait(value string) (string, error) {
	if value == "/img/characters/assistant.jpg" || value == "/img/characters/seraphina.jpg" {
		return value, nil
	}
	if value == "" {
		return "", nil
	}
	const prefix = "data:image/jpeg;base64,"
	if !strings.HasPrefix(value, prefix) || len(value) > 350000 {
		zap.L().Warn("character portrait rejected", zap.String("reason", "format or size"))
		return "", domain.ErrInvalid
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil {
		zap.L().Warn("character portrait rejected", zap.String("reason", "base64"), zap.Error(err))
		return "", domain.ErrInvalid
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width > 1024 || cfg.Height > 1024 {
		zap.L().Warn("character portrait rejected", zap.String("reason", "dimensions"), zap.Error(err))
		return "", domain.ErrInvalid
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		zap.L().Warn("character portrait rejected", zap.String("reason", "decode"), zap.Error(err))
		return "", domain.ErrInvalid
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		zap.L().Error("character portrait encode failed", zap.Error(err))
		return "", domain.ErrInvalid
	}
	encoded := prefix + base64.StdEncoding.EncodeToString(out.Bytes())
	if len(encoded) > 350000 {
		zap.L().Warn("character portrait rejected", zap.String("reason", "encoded size"))
		return "", domain.ErrInvalid
	}
	return encoded, nil
}

// cleanPortrait 保留内部调用兼容。
func cleanPortrait(value string) (string, error) { return CleanPortrait(value) }
