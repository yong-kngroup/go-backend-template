// Package shared 定义多个领域共享的值对象和技术无关端口。
//
// 这些端口只能表达领域所需的能力，不应泄露 Gin、GORM、Kafka 或 Redis 等具体类型。
package shared
