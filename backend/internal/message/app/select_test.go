// select_test.go 验证候选选择在缺失编号或仓储能力时安全失败。
package app

import (
	"context"
	"errors"
	"testing"
)

// TestSelectBounds 验证无效编号在访问仓储前被拒绝。
func TestSelectBounds(t *testing.T) {
	write := NewWrite(&baseRepo{}, nil)
	for _, ids := range [][3]uint64{{0, 8, 21}, {7, 0, 21}, {7, 8, 0}} {
		_, err := write.SelectVariant(context.Background(), ids[0], ids[1], ids[2])
		if !errors.Is(err, ErrQuery) {
			t.Fatalf("ids=%v error=%v", ids, err)
		}
	}
	if _, err := write.SelectVariant(context.Background(), 7, 8, 21); err == nil {
		t.Fatal("missing selector accepted")
	}
}
