// Package consumption 定义异步消息消费记录的状态和持久化契约。
//
// 消费记录以 consumer group 与 message key 唯一标识，用于跨进程去重、处理锁和
// 重试次数统计。它不提供恰好一次投递保证，而是让消费者在至少一次投递下安全恢复。
package consumption
