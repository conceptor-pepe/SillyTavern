// create.go 负责校验并创建角色资料。
package app

import (
	"ai-chat/backend/internal/character/domain"
	"context"
	"strings"

	"go.uber.org/zap"
)

// Creator 保存角色创建依赖。
type Creator struct{ repo domain.CreatorRepo }

// NewCreator 创建角色应用服务。
func NewCreator(repo domain.CreatorRepo) *Creator { return &Creator{repo: repo} }

// Create 校验当前用户输入并创建自己的角色。
func (s *Creator) Create(ctx context.Context, item domain.Character) (domain.Character, error) {
	if err := Validate(&item); err != nil {
		zap.L().Warn("character validation rejected", zap.Uint64("user_id", item.UserID), zap.Error(err))
		return domain.Character{}, err
	}
	created, err := s.repo.Create(ctx, item)
	if err != nil {
		zap.L().Error("character create failed", zap.Uint64("user_id", item.UserID), zap.Error(err))
		return domain.Character{}, err
	}
	return created, nil
}

// Validate 统一创建和更新校验，避免修改绕过角色约束。
func Validate(item *domain.Character) error {
	item.Name = strings.TrimSpace(item.Name)
	if item.UserID == 0 || item.Name == "" || len([]rune(item.Name)) > 128 {
		return domain.ErrInvalid
	}
	item.Description = strings.TrimSpace(item.Description)
	item.Personality = strings.TrimSpace(item.Personality)
	item.Scenario = strings.TrimSpace(item.Scenario)
	item.FirstMessage = strings.TrimSpace(item.FirstMessage)
	return cleanProfile(item)
}
