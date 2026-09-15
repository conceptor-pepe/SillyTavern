// favorite_test.go 验证收藏用例的参数校验和仓储错误传递。
package app

import (
	"context"
	"errors"
	"testing"
)

// favoriteRepoStub 记录收藏状态并返回预设错误。
type favoriteRepoStub struct {
	on    bool
	calls int
	err   error
}

// Set 记录收藏状态。
func (r *favoriteRepoStub) Set(_ context.Context, _, _ uint64, on bool) error {
	r.calls++
	r.on = on
	return r.err
}

// TestFavoriteSet 验证收藏状态会传给仓储。
func TestFavoriteSet(t *testing.T) {
	repo := &favoriteRepoStub{}
	if err := NewFavorite(repo).Set(context.Background(), 1, 2, true); err != nil {
		t.Fatal(err)
	}
	if !repo.on || repo.calls != 1 {
		t.Fatalf("on=%v calls=%d", repo.on, repo.calls)
	}
}

// TestFavoriteReject 验证无效编号不会访问仓储。
func TestFavoriteReject(t *testing.T) {
	repo := &favoriteRepoStub{}
	if err := NewFavorite(repo).Set(context.Background(), 0, 2, true); err == nil || repo.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, repo.calls)
	}
}

// TestFavoriteError 验证仓储错误不会被吞掉。
func TestFavoriteError(t *testing.T) {
	want := errors.New("favorite failed")
	repo := &favoriteRepoStub{err: want}
	if err := NewFavorite(repo).Set(context.Background(), 1, 2, false); !errors.Is(err, want) {
		t.Fatalf("err=%v want=%v", err, want)
	}
}
