// Package redis 提供可复用的 Redis 客户端初始化和 OpenTelemetry 追踪钩子。
//
// 它不包含缓存 key、TTL、回填或失效策略；这些与业务数据相关的规则属于 Repository。
package redis
