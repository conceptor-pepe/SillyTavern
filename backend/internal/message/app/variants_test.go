// variants_test.go 验证候选查询的输入、归属和底层错误边界。
package app

import (
	"context"
	"errors"
	"testing"

	"ai-chat/backend/internal/message/domain"
)

// variantRepo 隔离候选查询，记录分页和消息可见性检查顺序。
type variantRepo struct {
	baseRepo
	domain.VariantRepo
	findErr error
	listErr error
	called  bool
}

// Find 模拟消息归属查询失败或成功。
func (r *variantRepo) Find(context.Context, uint64, uint64) (domain.Message, error) {
	return domain.Message{ID: 12}, r.findErr
}

// ListVariants 仅允许测试约定的用户、消息和分页范围。
func (r *variantRepo) ListVariants(_ context.Context, uid, id uint64, page, size int) ([]domain.Variant, int64, error) {
	r.called = true
	if uid != 7 || id != 12 || page != 2 || size != 1 {
		return nil, 0, errors.New("unexpected query")
	}
	return []domain.Variant{{ID: 9, MessageID: id, VariantNo: 1}}, 2, r.listErr
}

// TestVariantsQuery 验证成功分页和归属失败时不得继续查询候选。
func TestVariantsQuery(t *testing.T) {
	repo := &variantRepo{}
	query := NewQuery(repo, nil)
	items, total, err := query.Variants(context.Background(), 7, 12, 2, 1)
	if err != nil || len(items) != 1 || total != 2 || !repo.called {
		t.Fatalf("items=%v total=%d err=%v", items, total, err)
	}
	for _, cause := range []error{domain.ErrNotFound, errors.New("database unavailable")} {
		repo.findErr, repo.called = cause, false
		_, _, err := query.Variants(context.Background(), 7, 12, 2, 1)
		if !errors.Is(err, cause) || repo.called {
			t.Fatalf("err=%v called=%v", err, repo.called)
		}
	}
	repo.findErr, repo.listErr = nil, errors.New("candidate query failed")
	_, _, err = query.Variants(context.Background(), 7, 12, 2, 1)
	if !errors.Is(err, repo.listErr) {
		t.Fatalf("err=%v", err)
	}
}

// TestVariantsBounds 验证无效请求在访问仓储前被拒绝。
func TestVariantsBounds(t *testing.T) {
	for _, in := range []struct {
		uid, id    uint64
		page, size int
	}{{0, 12, 1, 20}, {7, 0, 1, 20}, {7, 12, 0, 20}, {7, 12, 1, 0}, {7, 12, 1, 101}} {
		query := NewQuery(nil, nil)
		_, _, err := query.Variants(context.Background(), in.uid, in.id, in.page, in.size)
		if !errors.Is(err, ErrQuery) {
			t.Fatalf("input=%+v err=%v", in, err)
		}
	}
	_, _, err := NewQuery(&baseRepo{}, nil).Variants(context.Background(), 7, 12, 1, 20)
	if err == nil {
		t.Fatal("missing capability accepted")
	}
}
