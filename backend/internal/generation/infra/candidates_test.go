// candidates_test.go 验证候选行转换的编号、归属和扩展字段规则，不替代数据库事务测试。
package infra

import (
	"testing"

	msgdomain "ai-chat/backend/internal/message/domain"
)

// TestCandidateRows 验证候选使用事务创建的消息编号而非调用方编号。
func TestCandidateRows(t *testing.T) {
	rows, err := candidateRows(7, []msgdomain.Variant{
		{MessageID: 99, VariantNo: 0, Content: "A"},
		{MessageID: 98, VariantNo: 1, Content: "B", ExtraData: []byte(`{"source":"test"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].MessageID != 7 || rows[1].MessageID != 7 {
		t.Fatalf("rows=%+v", rows)
	}
	if rows[0].ExtraData != "{}" || rows[1].ExtraData != `{"source":"test"}` {
		t.Fatalf("rows=%+v", rows)
	}
}

// TestInvalidRows 验证重复编号、负编号或非法 JSON 不生成可写入行。
func TestInvalidRows(t *testing.T) {
	cases := [][]msgdomain.Variant{
		{{VariantNo: -1}},
		{{VariantNo: 0}, {VariantNo: 0}},
		{{VariantNo: 0, ExtraData: []byte("{bad}")}},
	}
	for _, items := range cases {
		rows, err := candidateRows(7, items)
		if err == nil || rows != nil {
			t.Fatalf("rows=%+v err=%v", rows, err)
		}
	}
}
