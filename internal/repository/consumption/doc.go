// Package consumption 实现消息消费记录的 PostgreSQL 持久化。
//
// Begin 使用唯一约束和行锁原子地领取、拒绝或恢复一条消息；实现必须保持该原子性，
// 否则多个 worker 可能同时执行相同副作用。
package consumption
