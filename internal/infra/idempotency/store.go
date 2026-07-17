package idempotency

import (
	"context"
	"time"
)

// Record 表示已领取的幂等请求及其可重放响应。
type Record struct {
	ID           uint
	RequestHash  string
	ResponseBody []byte
	StatusCode   int
	CompletedAt  *time.Time
}

// Store 原子地领取幂等键并保存首次请求的响应。
//
// 幂等命名空间由 actor、method、route 和 key 组成；requestHash 用于拒绝同一 key
// 对应不同请求体的情况。Claim 返回 claimed=false 时，调用方必须根据记录状态重放、
// 拒绝或报告处理中，而不能再次执行业务逻辑。
type Store interface {
	// Claim 返回已创建的记录和 true，或返回既有记录和 false。
	Claim(context.Context, uint, string, string, string, string) (*Record, bool, error)
	// Complete 只完成尚未完成的记录，保存原始响应以供同一请求重放。
	Complete(context.Context, uint, []byte, int) error
}
