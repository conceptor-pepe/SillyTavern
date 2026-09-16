// create.go 负责校验并创建角色资料。
package app

import (
	"context"
	"strings"

	"ai-chat/backend/internal/character/domain"
)

// Creator 保存角色创建依赖。
type Creator struct{ repo domain.CreatorRepo }

// NewCreator 创建角色应用服务。
func NewCreator(repo domain.CreatorRepo) *Creator { return &Creator{repo: repo} }

// Create 校验当前用户输入并创建自己的角色。
func (s *Creator) Create(ctx context.Context, item domain.Character) (domain.Character, error) {
	item.Name = strings.TrimSpace(item.Name)
	if item.UserID == 0 || item.Name == "" || len([]rune(item.Name)) > 128 {
		return domain.Character{}, domain.ErrInvalid
	}
	item.Description = strings.TrimSpace(item.Description)
	item.Personality = strings.TrimSpace(item.Personality)
	item.Scenario = strings.TrimSpace(item.Scenario)
	item.FirstMessage = strings.TrimSpace(item.FirstMessage)
	if err := cleanProfile(&item); err != nil {
		return domain.Character{}, err
	}
	return s.repo.Create(ctx, item)
}
