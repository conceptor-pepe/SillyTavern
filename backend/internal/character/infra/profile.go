// profile.go 映射角色展示资料，复用现有 JSON 字段而不修改数据库结构。
package infra

import (
	"encoding/json"
	"path/filepath"

	"ai-chat/backend/internal/character/domain"
	"ai-chat/backend/internal/model"
)

// profileData 保存展示字段，与旧迁移来源字段兼容。
type profileData struct {
	Portrait   string `json:"portrait,omitempty"`
	Gender     string `json:"gender,omitempty"`
	Age        string `json:"age,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
}

// readProfile 读取可选资料，旧记录没有新增字段时仍能正常展示。
func readProfile(row model.Character, item *domain.Character) {
	var extra profileData
	if err := json.Unmarshal([]byte(row.ExtraData), &extra); err == nil {
		item.Portrait, item.Gender, item.Age = extra.Portrait, extra.Gender, extra.Age
	}
	if err := json.Unmarshal([]byte(row.Tags), &item.Tags); err != nil {
		item.Tags = []string{}
	}
	if item.Portrait != "" {
		return
	}
	switch filepath.Base(extra.SourcePath) {
	case "default_Seraphina.png":
		item.Portrait = "/img/characters/seraphina.jpg"
	case "default_Assistant.png":
		item.Portrait = "/img/characters/assistant.jpg"
	}
}
