// Package outbox 提供本地事务消息的持久化、事件总线和发布任务。
//
// 业务 Usecase 通过 shared.EventBus 写入事件；cron 扫描已提交但尚未确认发布的事件。
// 本包提供至少一次投递语义，消费者必须能够处理重复消息。
package outbox
