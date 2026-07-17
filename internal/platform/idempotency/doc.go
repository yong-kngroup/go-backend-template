// Package idempotency 提供 HTTP 写请求的幂等记录存储。
//
// 它保存请求指纹和首次响应，用于重放已完成请求并拒绝同一 key 的不同请求体。
package idempotency
