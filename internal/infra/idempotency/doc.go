// Package idempotency 以 PostgreSQL 唯一约束实现 HTTP 写请求的幂等记录。
//
// 记录由调用者身份、HTTP 方法、路由和 Idempotency-Key 唯一确定；请求体哈希用于
// 检测同一 key 被用于不同请求。已完成记录保存响应以便重放，未完成记录表示请求仍在执行。
package idempotency
