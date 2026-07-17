// Package postgres 提供本项目运行时使用的 GORM、PostgreSQL 连接和 SQL 迁移工具。
//
// 本包不依赖其他应用内部层；Usecase 与 Repository 之间的事务传播由 internal/repository 负责。
package postgres
