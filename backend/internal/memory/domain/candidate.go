// candidate.go 定义用户确认后才生效的自动记忆候选。
package domain

import "context"

// Candidate 是从一条已确认分支提取的待审核记忆。
type Candidate struct {
	ID          uint64  `json:"id,string"`
	UserID      uint64  `json:"-"`
	ChatID      uint64  `json:"chat_id,string"`
	CompanionID uint64  `json:"companion_id,string,omitempty"`
	SourceEndID uint64  `json:"source_end_id,string"`
	Scope       string  `json:"scope"`
	Content     string  `json:"content"`
	Evidence    string  `json:"evidence"`
	Status      string  `json:"status"`
	MemoryID    *uint64 `json:"memory_id,string,omitempty"`
	Fingerprint string  `json:"-"`
}

// Proposal 是模型输出的未持久化提议。
type Proposal struct {
	Scope    string `json:"scope"`
	Content  string `json:"content"`
	Evidence string `json:"evidence"`
}

// CandidateScope 是服务端从会话归属解析出的允许范围。
type CandidateScope struct {
	UserID      uint64
	ChatID      uint64
	CompanionID uint64
	Story       bool
}

// CandidateRepo 保存候选并原子执行接受或拒绝。
type CandidateRepo interface {
	CandidateScope(context.Context, uint64, uint64) (CandidateScope, error)
	ListCandidates(context.Context, uint64, uint64) ([]Candidate, error)
	SaveCandidates(context.Context, []Candidate) ([]Candidate, error)
	AcceptCandidate(context.Context, uint64, uint64) (Candidate, error)
	RejectCandidate(context.Context, uint64, uint64) (Candidate, error)
}

// CandidateExtractor 从有界分支文本中提出结构化记忆。
type CandidateExtractor interface {
	Extract(context.Context, string, bool, bool) ([]Proposal, error)
}
