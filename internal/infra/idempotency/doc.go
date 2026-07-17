// Package idempotency 定义 HTTP 写请求幂等存储的技术契约。
//
// 记录由调用者身份、HTTP 方法、路由和 Idempotency-Key 唯一确定；请求体哈希用于
// 检测同一 key 被用于不同请求。已完成记录保存响应以便重放，未完成记录表示请求仍在执行。
// PostgreSQL 的表模型和读写实现分别位于 model/idempotency 与 repository/idempotency。
package idempotency
