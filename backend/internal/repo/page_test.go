// page_test.go 验证分页参数的默认值、边界和偏移量。
package repo

import "testing"

// TestNewPage 验证非法分页参数会被修正为安全范围。
func TestNewPage(t *testing.T) {
	tests := []struct {
		name string
		no   int
		size int
		want Page
	}{
		{name: "default", no: 0, size: 0, want: Page{No: 1, Size: 20}},
		{name: "negative", no: -1, size: -2, want: Page{No: 1, Size: 20}},
		{name: "large", no: 2, size: 200, want: Page{No: 2, Size: 100}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NewPage(test.no, test.size); got != test.want {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

// TestOffset 验证分页页码转换为数据库偏移量。
func TestOffset(t *testing.T) {
	page := NewPage(3, 20)
	if got := page.Offset(); got != 40 {
		t.Fatalf("got %d, want 40", got)
	}
}
