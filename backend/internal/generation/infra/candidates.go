// candidates.go 在生成完成事务内写入候选，失败由外层事务统一回滚。
package infra

import (
	"encoding/json"
	"errors"

	msgdomain "ai-chat/backend/internal/message/domain"
	"ai-chat/backend/internal/model"
	"gorm.io/gorm"
)

// saveCandidates 使用新消息编号绑定候选，不接受调用方传入的消息归属。
func saveCandidates(tx *gorm.DB, messageID uint64, items []msgdomain.Variant) ([]msgdomain.Variant, error) {
	rows, err := candidateRows(messageID, items)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	if err := tx.Create(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]msgdomain.Variant, 0, len(rows))
	for _, row := range rows {
		out = append(out, msgdomain.Variant{
			ID: row.ID, MessageID: row.MessageID, VariantNo: row.VariantNo,
			Content: row.Content, ExtraData: []byte(row.ExtraData),
		})
	}
	return out, nil
}

// candidateRows 拒绝重复或负候选编号及非法 JSON，避免事务提交不一致数据。
func candidateRows(messageID uint64, items []msgdomain.Variant) ([]model.MessageVariant, error) {
	rows := make([]model.MessageVariant, 0, len(items))
	seen := make(map[int]bool)
	for _, item := range items {
		if item.VariantNo < 0 || seen[item.VariantNo] {
			return nil, errors.New("invalid candidate index")
		}
		seen[item.VariantNo] = true
		extra := string(item.ExtraData)
		if extra == "" {
			extra = "{}"
		}
		if !json.Valid([]byte(extra)) {
			return nil, errors.New("invalid candidate extra data")
		}
		rows = append(rows, model.MessageVariant{
			MessageID: messageID, VariantNo: item.VariantNo,
			Content: item.Content, ExtraData: extra,
		})
	}
	return rows, nil
}
