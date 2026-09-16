// profile_test.go 验证不可信封面和标签不能绕过角色创建校验。
package app

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"strings"
	"testing"

	"ai-chat/backend/internal/character/domain"
)

// TestProfileRejects 确保非法资料不会调用持久化仓储。
func TestProfileRejects(t *testing.T) {
	cases := []domain.Character{
		{Portrait: "https://example.com/track.jpg"},
		{Portrait: "data:image/jpeg;base64,YmFk"},
		{Portrait: strings.Repeat("x", 350001)},
		{Tags: []string{"日常", "日常"}},
		{Tags: []string{strings.Repeat("长", 25)}},
		{Tags: make([]string, 10)},
		{Gender: "invalid"},
		{Age: "123456789"},
		{Personality: strings.Repeat("x", 60001)},
	}
	for i, item := range cases {
		repo := &fakeCreator{}
		item.Name, item.UserID = "角色", 7
		_, err := NewCreator(repo).Create(t.Context(), item)
		if err != domain.ErrInvalid || repo.item.Name != "" {
			t.Fatalf("case %d: err=%v unexpected write=%v", i, err, repo.item.Name != "")
		}
	}
}

// TestPortraitLimits 验证有效图片保留像素且超出尺寸的 JPEG 被拒绝。
func TestPortraitLimits(t *testing.T) {
	for _, width := range []int{8, 1025} {
		var data bytes.Buffer
		if err := jpeg.Encode(&data, image.NewRGBA(image.Rect(0, 0, width, 8)), nil); err != nil {
			t.Fatal(err)
		}
		input := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data.Bytes())
		out, err := cleanPortrait(input)
		if width > 1024 {
			if err != domain.ErrInvalid {
				t.Fatal("oversized image accepted")
			}
			continue
		}
		if err != nil || !strings.HasPrefix(out, "data:image/jpeg;base64,") {
			t.Fatalf("valid JPEG failed: %v", err)
		}
	}
}
