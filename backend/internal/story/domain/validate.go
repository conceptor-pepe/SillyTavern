// validate.go 限制作品输入和引用，避免非法发言人或无界设定进入模型。
package domain

import (
	"regexp"
	"strings"
)

var localID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Validate 校验首版单角色作品和有序开场。
// @param d 故事定义
// @return 校验错误
func Validate(d Definition) error {
	if d.SchemaVersion != 1 || strings.TrimSpace(d.Title) == "" || len(d.Title) > 240 {
		return ErrInvalid
	}
	if len(d.Hook) > 600 || len(d.World) > 12000 || len(d.Examples) > 6000 || len(d.Cover) > 350000 {
		return ErrInvalid
	}
	if len(d.Cast) != 1 || len(d.Opening) > 12 || len(d.Tags) > 16 || len(d.Lore) > 32 {
		return ErrInvalid
	}
	c := d.Cast[0]
	if !localID.MatchString(c.ID) || strings.TrimSpace(c.Name) == "" || len(c.Name) > 120 {
		return ErrInvalid
	}
	if len(c.Description) > 6000 || len(c.Personality) > 4000 || len(c.Portrait) > 350000 || len(c.Gender) > 64 || len(c.Age) > 64 {
		return ErrInvalid
	}
	for _, tag := range d.Tags {
		if strings.TrimSpace(tag) == "" || len(tag) > 80 {
			return ErrInvalid
		}
	}
	if !validOpening(d.Opening, c.ID) || !validLore(d.Lore) {
		return ErrInvalid
	}
	return nil
}

func validOpening(items []Segment, speaker string) bool {
	seen := make(map[string]bool, len(items))
	total := 0
	for _, s := range items {
		if !localID.MatchString(s.ID) || seen[s.ID] || strings.TrimSpace(s.Text) == "" || len(s.Text) > 4000 {
			return false
		}
		if (s.Kind != "narration" || s.SpeakerID != "") && (s.Kind != "dialogue" || s.SpeakerID != speaker) {
			return false
		}
		seen[s.ID] = true
		total += len(s.Text)
	}
	return total <= 12000
}

func validLore(items []Lore) bool {
	total := 0
	for _, item := range items {
		if strings.TrimSpace(item.Content) == "" || len(item.Content) > 4000 || len(item.Keywords) > 16 {
			return false
		}
		if !item.Pinned && len(item.Keywords) == 0 {
			return false
		}
		if !validKeys(item.Keywords) {
			return false
		}
		total += len(item.Content)
	}
	return total <= 16000
}

func validKeys(keys []string) bool {
	for _, key := range keys {
		if strings.TrimSpace(key) == "" || len(key) > 128 {
			return false
		}
	}
	return true
}
