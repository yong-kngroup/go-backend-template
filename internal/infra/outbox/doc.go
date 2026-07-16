// Package outbox 实现领域事件写入本地 Outbox 表的基础设施。
//
// EventBus 不直接向 Kafka 发布消息；它使用调用 context 中的事务连接写入事件，确保
// 业务状态与待发布事件同时提交或回滚。
package outbox
