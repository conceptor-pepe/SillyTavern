// page.go 定义列表查询统一使用的分页参数和结果。
package repo

// Page 保存经过边界修正的分页参数。
type Page struct {
	No   int
	Size int
}

// PageResult 保存分页数据和总记录数。
type PageResult[T any] struct {
	Items []T
	Total int64
	No    int
	Size  int
}

// NewPage 创建分页参数，限制页码和单页数量范围。
func NewPage(no, size int) Page {
	if no < 1 {
		no = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return Page{No: no, Size: size}
}

// Offset 返回当前页对应的数据库偏移量。
func (p Page) Offset() int {
	return (p.No - 1) * p.Size
}
