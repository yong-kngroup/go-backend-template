// Package database 提供 PostgreSQL 连接、事务透传和数据库迁移基础设施。
//
// 事务通过 context 在 Usecase 和 Repository 之间传播。Repository 必须使用 DB 从
// context 取得当前事务连接，才能与同一用例中的其他写入和 Outbox 保持原子性。
package database
