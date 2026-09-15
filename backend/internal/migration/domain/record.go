// record.go 定义旧 JSONL 聊天记录的迁移输入结构。
package domain

import (
	"encoding/json"
	"errors"
)

// ErrMissing 表示迁移目标记录不存在，可安全创建。
var ErrMissing = errors.New("migration record missing")

// ErrSkip 表示旧存储中的非业务记录，例如聊天元数据或头像 KV。
var ErrSkip = errors.New("migration line skipped")

// Message 保存一条旧聊天消息及未识别字段。
type Message struct {
	Role      string
	Content   string
	Name      string
	ExtraData json.RawMessage
}
